package recovery

import (
	"context"
	"fmt"
	"time"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/store"
	"eks-bluegreen-cli/pkg/models"
)

type Recoverer struct {
	awsClient aws.Client
	store     store.Store
}

func NewRecoverer(awsClient aws.Client, st store.Store) *Recoverer {
	return &Recoverer{
		awsClient: awsClient,
		store:     st,
	}
}

// RecoverTraffic rolls back traffic routing from Green cluster back to Blue cluster.
func (r *Recoverer) RecoverTraffic(ctx context.Context, jobID, zoneID, recordName string) error {
	job, err := r.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	step := &models.ExecutionStep{
		JobID:     jobID,
		StepName:  "RollbackTrafficToBlue",
		Status:    models.StepStatusRunning,
		StartedAt: time.Now().UTC(),
	}
	_ = r.store.SaveExecutionStep(ctx, step)

	// Fetch Blue cluster info
	blueCl, err := r.awsClient.DescribeCluster(ctx, job.SourceCluster)
	if err != nil {
		step.Status = models.StepStatusFailed
		step.ErrorMessage = fmt.Sprintf("Failed to query Blue cluster %s: %v", job.SourceCluster, err)
		_ = r.store.SaveExecutionStep(ctx, step)
		return fmt.Errorf("failed to describe source cluster for rollback: %w", err)
	}

	// Switch traffic back to Blue
	if zoneID != "" && recordName != "" {
		if err := r.awsClient.UpdateDNSRecord(ctx, zoneID, recordName, blueCl.Endpoint); err != nil {
			step.Status = models.StepStatusFailed
			step.ErrorMessage = err.Error()
			_ = r.store.SaveExecutionStep(ctx, step)
			return fmt.Errorf("failed to revert DNS record to Blue: %w", err)
		}
	}

	now := time.Now().UTC()
	step.Status = models.StepStatusCompleted
	step.CompletedAt = &now
	step.OutputReference = fmt.Sprintf("Traffic successfully rolled back to Blue cluster (%s)", blueCl.Endpoint)
	_ = r.store.SaveExecutionStep(ctx, step)

	job.CurrentPhase = models.PhaseRecovered
	job.Status = models.StatusCompleted
	if err := r.store.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}
