package provisioner

import (
	"context"
	"fmt"
	"time"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/security"
	"eks-bluegreen-cli/internal/store"
	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

type Provisioner struct {
	awsClient aws.Client
	store     store.Store
}

func NewProvisioner(awsClient aws.Client, st store.Store) *Provisioner {
	return &Provisioner{
		awsClient: awsClient,
		store:     st,
	}
}

// ProvisionGreenInfrastructure handles provisioning Green cluster, node groups, and addons.
func (p *Provisioner) ProvisionGreenInfrastructure(ctx context.Context, job *models.UpgradeJob, plan *models.PlanReport, dryRun bool) error {
	// Guardrail: Assert Target != Source (Blue)
	if err := security.AssertSafeOperation(job.TargetCluster, job.SourceCluster, "provisioning"); err != nil {
		return err
	}

	if dryRun {
		// In dry-run mode, we do NOT execute API calls that create resources.
		return nil
	}

	// 1. Step: Provision EKS Control Plane
	stepCluster := &models.ExecutionStep{
		JobID:     job.ID,
		StepName:  "CreateEKSControlPlane",
		Status:    models.StepStatusRunning,
		StartedAt: time.Now().UTC(),
	}
	_ = p.store.SaveExecutionStep(ctx, stepCluster)

	// Check if already created (resumability)
	existingCluster, _ := p.awsClient.DescribeCluster(ctx, job.TargetCluster)
	var clusterInfo *models.EKSClusterInfo
	if existingCluster == nil {
		// Need source cluster VPC/role info
		sourceCluster, err := p.awsClient.DescribeCluster(ctx, job.SourceCluster)
		if err != nil {
			stepCluster.Status = models.StepStatusFailed
			stepCluster.ErrorMessage = err.Error()
			_ = p.store.SaveExecutionStep(ctx, stepCluster)
			return fmt.Errorf("failed to get source cluster info: %w", err)
		}

		created, err := p.awsClient.CreateCluster(ctx, aws.CreateClusterParams{
			Name:             job.TargetCluster,
			Version:          job.TargetVersion,
			RoleArn:          sourceCluster.RoleArn,
			SubnetIDs:        sourceCluster.SubnetIDs,
			SecurityGroupIDs: sourceCluster.SecurityGroupIDs,
			Tags: map[string]string{
				"ManagedBy":   "eks-bluegreen-cli",
				"Environment": "Green",
				"JobID":       job.ID,
			},
		})
		if err != nil {
			stepCluster.Status = models.StepStatusFailed
			stepCluster.ErrorMessage = err.Error()
			_ = p.store.SaveExecutionStep(ctx, stepCluster)
			return fmt.Errorf("failed to create green cluster: %w", err)
		}
		clusterInfo = created
	} else {
		clusterInfo = existingCluster
	}

	now := time.Now().UTC()
	stepCluster.Status = models.StepStatusCompleted
	stepCluster.CompletedAt = &now
	stepCluster.OutputReference = clusterInfo.Arn
	_ = p.store.SaveExecutionStep(ctx, stepCluster)

	// 2. Step: Create Node Groups
	for _, rp := range plan.ResourcePlans {
		if rp.ResourceType == "NodeGroup" {
			stepNG := &models.ExecutionStep{
				JobID:     job.ID,
				StepName:  "CreateNodeGroup:" + rp.ResourceName,
				Status:    models.StepStatusRunning,
				StartedAt: time.Now().UTC(),
			}
			_ = p.store.SaveExecutionStep(ctx, stepNG)

			// Fetch source node groups for spec replication
			srcNGs, _ := p.awsClient.ListNodeGroups(ctx, job.SourceCluster)
			var baseNG models.NodeGroupInfo
			if len(srcNGs) > 0 {
				baseNG = srcNGs[0]
			} else {
				baseNG = models.NodeGroupInfo{
					InstanceTypes: []string{"m5.large"},
					DesiredSize:   2,
					MinSize:       1,
					MaxSize:       4,
					AmiType:       "AL2_x86_64",
				}
			}

			newNG := models.NodeGroupInfo{
				Name:          rp.ResourceName,
				ClusterName:   job.TargetCluster,
				InstanceTypes: baseNG.InstanceTypes,
				DesiredSize:   baseNG.DesiredSize,
				MinSize:       baseNG.MinSize,
				MaxSize:       baseNG.MaxSize,
				Subnets:       clusterInfo.SubnetIDs,
				NodeRole:      baseNG.NodeRole,
				AmiType:       baseNG.AmiType,
				Labels: map[string]string{
					"environment": "green",
					"job-id":      job.ID,
				},
			}

			if err := p.awsClient.CreateNodeGroup(ctx, job.TargetCluster, newNG); err != nil {
				stepNG.Status = models.StepStatusFailed
				stepNG.ErrorMessage = err.Error()
				_ = p.store.SaveExecutionStep(ctx, stepNG)
				return fmt.Errorf("failed to create node group %s: %w", rp.ResourceName, err)
			}

			ngNow := time.Now().UTC()
			stepNG.Status = models.StepStatusCompleted
			stepNG.CompletedAt = &ngNow
			_ = p.store.SaveExecutionStep(ctx, stepNG)
		}
	}

	// 3. Step: Install Addons
	for _, rp := range plan.ResourcePlans {
		if rp.ResourceType == "EKSAddon" {
			stepAddon := &models.ExecutionStep{
				JobID:     job.ID,
				StepName:  "InstallAddon:" + rp.ResourceName,
				Status:    models.StepStatusRunning,
				StartedAt: time.Now().UTC(),
			}
			_ = p.store.SaveExecutionStep(ctx, stepAddon)

			if err := p.awsClient.CreateAddon(ctx, job.TargetCluster, rp.ResourceName, ""); err != nil {
				stepAddon.Status = models.StepStatusFailed
				stepAddon.ErrorMessage = err.Error()
				_ = p.store.SaveExecutionStep(ctx, stepAddon)
				return fmt.Errorf("failed to install addon %s: %w", rp.ResourceName, err)
			}

			addonNow := time.Now().UTC()
			stepAddon.Status = models.StepStatusCompleted
			stepAddon.CompletedAt = &addonNow
			_ = p.store.SaveExecutionStep(ctx, stepAddon)
		}
	}

	return nil
}

// AssertNoBlueClusterMutation ensures no provisioner action can target Blue cluster.
func AssertNoBlueClusterMutation(sourceCluster, targetCluster string) error {
	if sourceCluster == targetCluster {
		return apperrors.ErrBlueClusterProtected
	}
	return nil
}
