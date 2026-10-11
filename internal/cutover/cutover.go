package cutover

import (
	"context"
	"fmt"
	"time"

	"eks-bluegreen-cli/internal/approval"
	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/security"
	"eks-bluegreen-cli/internal/store"
	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

type Operator struct {
	awsClient aws.Client
	appMgr    *approval.Manager
	store     store.Store
}

func NewOperator(awsClient aws.Client, appMgr *approval.Manager, st store.Store) *Operator {
	return &Operator{
		awsClient: awsClient,
		appMgr:    appMgr,
		store:     st,
	}
}

// ExecuteCutover performs traffic cutover to Green cluster.
func (op *Operator) ExecuteCutover(ctx context.Context, jobID, zoneID, recordName, currentPlanHash, currentValHash string) error {
	job, err := op.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	// 1. Guardrail: Protect Blue cluster from destructive operations
	if err := security.ValidateBlueClusterProtection(job.TargetCluster, job.SourceCluster); err != nil {
		return err
	}

	// 2. Verify Approval & Tamper Check
	if err := op.appMgr.VerifyApproval(ctx, jobID, currentPlanHash, currentValHash); err != nil {
		return err
	}

	// 3. Record Cutover Start
	step := &models.ExecutionStep{
		JobID:     jobID,
		StepName:  "ExecuteTrafficCutover",
		Status:    models.StepStatusRunning,
		StartedAt: time.Now().UTC(),
	}
	_ = op.store.SaveExecutionStep(ctx, step)

	// Fetch green cluster endpoint
	greenCl, err := op.awsClient.DescribeCluster(ctx, job.TargetCluster)
	if err != nil {
		step.Status = models.StepStatusFailed
		step.ErrorMessage = err.Error()
		_ = op.store.SaveExecutionStep(ctx, step)
		return fmt.Errorf("failed to retrieve target cluster info: %w", err)
	}

	// Update traffic (e.g. Route53)
	if zoneID != "" && recordName != "" {
		if err := op.awsClient.UpdateDNSRecord(ctx, zoneID, recordName, greenCl.Endpoint); err != nil {
			step.Status = models.StepStatusFailed
			step.ErrorMessage = fmt.Sprintf("cutover failed, Blue traffic retained: %v", err)
			_ = op.store.SaveExecutionStep(ctx, step)
			return fmt.Errorf("traffic cutover failed: %w", err)
		}
	}

	// Complete step
	now := time.Now().UTC()
	step.Status = models.StepStatusCompleted
	step.CompletedAt = &now
	step.OutputReference = fmt.Sprintf("Traffic routed to Green cluster (%s)", greenCl.Endpoint)
	_ = op.store.SaveExecutionStep(ctx, step)

	// Update Job
	job.CurrentPhase = models.PhaseCutoverCompleted
	job.Status = models.StatusCompleted
	if err := op.store.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

// ProtectBlueClusterStrict prevents deletion of Blue cluster under all circumstances.
func ProtectBlueClusterStrict(clusterName, blueClusterName string) error {
	if clusterName == blueClusterName {
		return apperrors.ErrBlueClusterDeletion
	}
	return nil
}
