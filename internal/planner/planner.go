package planner

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/security"
	"eks-bluegreen-cli/pkg/models"
)

// Supported EKS versions validation
var supportedEKSVersions = []string{"1.28", "1.29", "1.30", "1.31"}

type Planner struct {
	awsClient aws.Client
}

func NewPlanner(awsClient aws.Client) *Planner {
	return &Planner{awsClient: awsClient}
}

// GeneratePlan creates a comprehensive upgrade plan comparing source cluster with target version.
func (p *Planner) GeneratePlan(ctx context.Context, jobID string, inspection *models.ClusterInspection, targetCluster, targetVersion, region string) (*models.PlanReport, error) {
	if inspection == nil {
		return nil, fmt.Errorf("inspection data cannot be nil")
	}

	report := &models.PlanReport{
		JobID:              jobID,
		SourceCluster:      inspection.Cluster.Name,
		SourceVersion:      inspection.Cluster.Version,
		TargetCluster:      targetCluster,
		TargetVersion:      targetVersion,
		Region:             region,
		Compatible:         true,
		BlockingReasons:    []string{},
		DeprecatedWarnings: inspection.DeprecatedAPIs,
		ResourcePlans:      []models.ResourcePlan{},
		CreatedAt:          time.Now().UTC(),
	}

	// 1. Validate Target EKS Version
	if !isVersionSupported(targetVersion) {
		report.Compatible = false
		report.BlockingReasons = append(report.BlockingReasons,
			fmt.Sprintf("Target version %q is not in recognized supported EKS versions (%s)",
				targetVersion, strings.Join(supportedEKSVersions, ", ")))
	}

	// Compare source and target version minor versions (e.g. 1.28 -> 1.30: max 2 minor versions skip warning/check)
	srcMinor, srcErr := parseMinorVersion(inspection.Cluster.Version)
	tgtMinor, tgtErr := parseMinorVersion(targetVersion)
	if srcErr == nil && tgtErr == nil {
		if tgtMinor <= srcMinor {
			report.Compatible = false
			report.BlockingReasons = append(report.BlockingReasons,
				fmt.Sprintf("Target version %s must be higher than current source version %s", targetVersion, inspection.Cluster.Version))
		} else if tgtMinor-srcMinor > 2 {
			report.BlockingReasons = append(report.BlockingReasons,
				fmt.Sprintf("Upgrading across %d minor versions (from %s to %s) requires multi-step migration testing",
					tgtMinor-srcMinor, inspection.Cluster.Version, targetVersion))
		}
	}

	// 2. Add-on compatibility check
	addonChecks, err := p.awsClient.DescribeAddonVersions(ctx, targetVersion)
	if err == nil {
		report.AddonCompatibility = addonChecks
	} else {
		// Fallback
		for _, a := range inspection.Addons {
			report.AddonCompatibility = append(report.AddonCompatibility, models.AddonCompatibilityCheck{
				Name:           a.Name,
				CurrentVersion: a.Version,
				TargetVersion:  "latest-compatible",
				IsCompatible:   true,
				Recommendation: fmt.Sprintf("Upgrade %s to target EKS %s compatible version", a.Name, targetVersion),
			})
		}
	}

	// 3. Subnet IP capacity check
	totalSubnetIPs := int32(0)
	for _, sub := range inspection.Network.Subnets {
		totalSubnetIPs += sub.AvailableIPs
	}
	if totalSubnetIPs < 30 {
		report.BlockingReasons = append(report.BlockingReasons,
			fmt.Sprintf("Insufficient available subnet IPs (%d available). Blue-Green duplication requires at least 30 free IPs.", totalSubnetIPs))
	}

	// 4. Classify resources to replicate, reconfigure, or exclude
	// Control Plane
	report.ResourcePlans = append(report.ResourcePlans, models.ResourcePlan{
		JobID:            jobID,
		ResourceType:     "EKSControlPlane",
		ResourceName:     targetCluster,
		Action:           models.ActionReplicate,
		Dependencies:     []string{"VPC:" + inspection.Cluster.VpcID},
		RiskLevel:        models.RiskLow,
		ValidationStatus: "PENDING",
		Notes:            fmt.Sprintf("Provision new Green EKS control plane v%s", targetVersion),
	})

	// Node Groups
	for _, ng := range inspection.NodeGroups {
		greenNGName := ng.Name + "-green"
		report.ResourcePlans = append(report.ResourcePlans, models.ResourcePlan{
			JobID:            jobID,
			ResourceType:     "NodeGroup",
			ResourceName:     greenNGName,
			Action:           models.ActionReplicate,
			Dependencies:     []string{"EKSControlPlane:" + targetCluster},
			RiskLevel:        models.RiskMedium,
			ValidationStatus: "PENDING",
			Notes:            fmt.Sprintf("Replicate nodegroup with %d %s instances for target EKS v%s", ng.DesiredSize, strings.Join(ng.InstanceTypes, ","), targetVersion),
		})
	}

	// Addons
	for _, a := range inspection.Addons {
		report.ResourcePlans = append(report.ResourcePlans, models.ResourcePlan{
			JobID:            jobID,
			ResourceType:     "EKSAddon",
			ResourceName:     a.Name,
			Action:           models.ActionReplicate,
			Dependencies:     []string{"EKSControlPlane:" + targetCluster},
			RiskLevel:        models.RiskLow,
			ValidationStatus: "PENDING",
			Notes:            fmt.Sprintf("Install addon %s on Green cluster", a.Name),
		})
	}

	// Workloads
	for _, d := range inspection.Workloads.Deployments {
		report.ResourcePlans = append(report.ResourcePlans, models.ResourcePlan{
			JobID:            jobID,
			ResourceType:     "Deployment",
			ResourceName:     d.Namespace + "/" + d.Name,
			Action:           models.ActionReplicate,
			Dependencies:     []string{"Namespace:" + d.Namespace},
			RiskLevel:        models.RiskLow,
			ValidationStatus: "PENDING",
			Notes:            fmt.Sprintf("Deploy %d replicas of %s", d.Replicas, d.Name),
		})
	}

	// StatefulSets
	for _, ss := range inspection.Workloads.StatefulSets {
		report.ResourcePlans = append(report.ResourcePlans, models.ResourcePlan{
			JobID:            jobID,
			ResourceType:     "StatefulSet",
			ResourceName:     ss.Namespace + "/" + ss.Name,
			Action:           models.ActionManualCheck,
			Dependencies:     []string{"StorageClass", "PVC"},
			RiskLevel:        models.RiskHigh,
			ValidationStatus: "REQUIRES_REVIEW",
			Notes:            "StatefulSet data migration must be handled via snapshot replication or database replica",
		})
	}

	// Ingresses
	for _, ing := range inspection.Workloads.Ingresses {
		report.ResourcePlans = append(report.ResourcePlans, models.ResourcePlan{
			JobID:            jobID,
			ResourceType:     "Ingress",
			ResourceName:     ing.Namespace + "/" + ing.Name,
			Action:           models.ActionReconfigure,
			Dependencies:     []string{"Service"},
			RiskLevel:        models.RiskMedium,
			ValidationStatus: "PENDING",
			Notes:            "Reconfigure Green Ingress with independent ALB target group before cutover",
		})
	}

	// PVCs
	for _, pvc := range inspection.Workloads.PVCs {
		report.ResourcePlans = append(report.ResourcePlans, models.ResourcePlan{
			JobID:            jobID,
			ResourceType:     "PersistentVolumeClaim",
			ResourceName:     pvc.Namespace + "/" + pvc.Name,
			Action:           models.ActionManualCheck,
			Dependencies:     []string{"StorageClass:" + pvc.StorageClass},
			RiskLevel:        models.RiskCritical,
			ValidationStatus: "REQUIRES_REVIEW",
			Notes:            fmt.Sprintf("EBS/EFS volume (%s) requires snapshot restore strategy; direct attachment across clusters prohibited", pvc.RequestedStorage),
		})
	}

	// 5. Cost Estimation
	nodeCount := 0
	for _, ng := range inspection.NodeGroups {
		nodeCount += int(ng.DesiredSize)
	}
	if nodeCount == 0 {
		nodeCount = 2
	}
	cpCost := 73.0                                // ~$0.10/hr
	workerCost := float64(nodeCount) * 0.096 * 730 // ~$70 per m5.large/mo
	report.EstimatedCost = models.CostEstimate{
		ControlPlaneMonthlyUSD:   cpCost,
		WorkerNodesMonthlyUSD:    workerCost,
		EstimatedTotalMonthlyUSD: cpCost + workerCost,
	}

	if len(report.BlockingReasons) > 0 {
		report.Compatible = false
	}

	// Compute canonical hash
	hash, err := security.ComputePlanHash(report)
	if err == nil {
		report.PlanHash = hash
	}

	return report, nil
}

func isVersionSupported(version string) bool {
	clean := strings.TrimPrefix(version, "v")
	for _, v := range supportedEKSVersions {
		if v == clean {
			return true
		}
	}
	return false
}

func parseMinorVersion(version string) (int, error) {
	clean := strings.TrimPrefix(version, "v")
	parts := strings.Split(clean, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid version format: %s", version)
	}
	return strconv.Atoi(parts[1])
}
