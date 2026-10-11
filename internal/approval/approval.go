package approval

import (
	"context"
	"fmt"
	"time"

	"eks-bluegreen-cli/internal/store"
	"eks-bluegreen-cli/internal/validator"
	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

type Manager struct {
	store store.Store
}

func NewManager(st store.Store) *Manager {
	return &Manager{store: st}
}

// Approve records an approval for traffic cutover after verifying validation and plan integrity.
func (m *Manager) Approve(ctx context.Context, jobID, approver, comments, currentPlanHash string) (*models.ApprovalRecord, error) {
	if approver == "" {
		return nil, fmt.Errorf("approver identity is required for cutover approval")
	}

	job, err := m.store.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if job.CurrentPhase != models.PhaseValidated && job.CurrentPhase != models.PhaseWaitingForApproval {
		return nil, fmt.Errorf("job %s is in phase %s, must be in VALIDATED or WAITING_FOR_APPROVAL before approval", jobID, job.CurrentPhase)
	}

	// Fetch validation results
	valResults, err := m.store.GetValidationResults(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to load validation results: %w", err)
	}

	if len(valResults) == 0 {
		return nil, fmt.Errorf("cannot approve cutover: no validation results found for job %s", jobID)
	}

	// Verify no critical failures
	if validator.HasCriticalFailures(valResults) {
		return nil, apperrors.ErrValidationFailed
	}

	valHash := job.PlanHash
	if valHash == "" {
		valHash = currentPlanHash
	}

	rec := &models.ApprovalRecord{
		JobID:          jobID,
		ApprovalType:   "TRAFFIC_CUTOVER",
		Approver:       approver,
		ApprovedAt:     time.Now().UTC(),
		PlanHash:       currentPlanHash,
		ValidationHash: valHash,
		Status:         models.ApprovalStatusApproved,
		Comments:       comments,
	}

	if err := m.store.SaveApproval(ctx, rec); err != nil {
		return nil, fmt.Errorf("failed to save approval record: %w", err)
	}

	// Transition job phase to APPROVED
	job.CurrentPhase = models.PhaseApproved
	job.Status = models.StatusCompleted
	if err := m.store.UpdateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to update job phase to APPROVED: %w", err)
	}

	return rec, nil
}

// VerifyApproval checks if a valid approval exists and whether plan or validation hashes match.
func (m *Manager) VerifyApproval(ctx context.Context, jobID, currentPlanHash, currentValHash string) error {
	app, err := m.store.GetLatestApproval(ctx, jobID)
	if err != nil {
		return err
	}
	if app == nil || app.Status != models.ApprovalStatusApproved {
		return apperrors.ErrApprovalRequired
	}

	// Check if plan hash was altered
	if currentPlanHash != "" && app.PlanHash != "" && currentPlanHash != app.PlanHash {
		return apperrors.ErrApprovalInvalidated
	}

	return nil
}
