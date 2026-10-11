package inspector

import (
	"context"
	"fmt"
	"time"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/deployer/k8s"
	"eks-bluegreen-cli/pkg/models"
)

type Inspector struct {
	awsClient aws.Client
	k8sClient k8s.K8sClient
}

func NewInspector(awsClient aws.Client, k8sClient k8s.K8sClient) *Inspector {
	return &Inspector{
		awsClient: awsClient,
		k8sClient: k8sClient,
	}
}

// Inspect collects cluster, network, node groups, addons, and workload configurations.
func (ins *Inspector) Inspect(ctx context.Context, clusterName string) (*models.ClusterInspection, error) {
	clInfo, err := ins.awsClient.DescribeCluster(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect cluster %q: %w", clusterName, err)
	}

	nodeGroups, err := ins.awsClient.ListNodeGroups(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect node groups: %w", err)
	}

	addons, err := ins.awsClient.ListAddons(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect addons: %w", err)
	}

	var netInfo *models.NetworkInfo
	if clInfo.VpcID != "" {
		netInfo, err = ins.awsClient.DescribeVpc(ctx, clInfo.VpcID)
		if err != nil {
			// Non-fatal, provide fallback
			netInfo = &models.NetworkInfo{
				VpcID:   clInfo.VpcID,
				Subnets: []models.SubnetDetail{},
			}
		}
	} else {
		netInfo = &models.NetworkInfo{}
	}

	var workloads *models.K8sWorkloadInfo
	if ins.k8sClient != nil {
		workloads, err = ins.k8sClient.InspectWorkloads(ctx)
		if err != nil {
			workloads = &models.K8sWorkloadInfo{}
		}
	} else {
		workloads = &models.K8sWorkloadInfo{}
	}

	// Analyze deprecated APIs
	deprecations := CheckDeprecatedAPIs(workloads, clInfo.Version)

	inspection := &models.ClusterInspection{
		Cluster:        *clInfo,
		NodeGroups:     nodeGroups,
		Addons:         addons,
		Network:        *netInfo,
		Workloads:      *workloads,
		DeprecatedAPIs: deprecations,
		InspectedAt:    time.Now().UTC(),
	}

	return inspection, nil
}

// CheckDeprecatedAPIs scans workload resources for deprecated APIs based on target versions.
func CheckDeprecatedAPIs(w *models.K8sWorkloadInfo, currentVersion string) []models.DeprecatedAPIWarning {
	var warnings []models.DeprecatedAPIWarning

	if w == nil {
		return warnings
	}

	// Check deployments APIVersion
	for _, d := range w.Deployments {
		if d.APIVersion == "extensions/v1beta1" || d.APIVersion == "apps/v1beta1" || d.APIVersion == "apps/v1beta2" {
			warnings = append(warnings, models.DeprecatedAPIWarning{
				ResourceKind:     "Deployment",
				APIVersion:       d.APIVersion,
				RemovedInVersion: "1.16",
				ReplacementAPI:   "apps/v1",
				AffectedObjects:  []string{d.Namespace + "/" + d.Name},
			})
		}
	}

	// Check Ingress APIVersion
	for _, ing := range w.Ingresses {
		if ing.Class == "" {
			warnings = append(warnings, models.DeprecatedAPIWarning{
				ResourceKind:     "Ingress",
				APIVersion:       "networking.k8s.io/v1",
				RemovedInVersion: "1.22",
				ReplacementAPI:   "spec.ingressClassName",
				AffectedObjects:  []string{ing.Namespace + "/" + ing.Name},
			})
		}
	}

	return warnings
}
