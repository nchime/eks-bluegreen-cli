package workflow

import (
	"context"
	"fmt"
	"time"

	"eks-bluegreen-cli/internal/store"
	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

type Engine struct {
	store store.Store
}

func NewEngine(st store.Store) *Engine {
	return &Engine{store: st}
}

var validTransitions = map[models.JobPhase][]models.JobPhase{
	models.PhaseInit:               {models.PhaseInspected},
	models.PhaseInspected:          {models.PhasePlanned},
	models.PhasePlanned:            {models.PhaseProvisioning},
	models.PhaseProvisioning:       {models.PhaseProvisioned, models.PhaseProvisioning}, // Allow retry
	models.PhaseProvisioned:        {models.PhaseSyncing},
	models.PhaseSyncing:            {models.PhaseSynced, models.PhaseSyncing}, // Allow retry
	models.PhaseSynced:             {models.PhaseValidating},
	models.PhaseValidating:         {models.PhaseValidated, models.PhaseValidating}, // Allow re-validation
	models.PhaseValidated:          {models.PhaseWaitingForApproval, models.PhaseApproved, models.PhaseValidating},
	models.PhaseWaitingForApproval: {models.PhaseApproved, models.PhaseValidating},
	models.PhaseApproved:           {models.PhaseCutoverInProgress, models.PhaseValidating},
	models.PhaseCutoverInProgress:  {models.PhaseCutoverCompleted, models.PhaseApproved},
	models.PhaseCutoverCompleted:   {models.PhaseRecovered},
	models.PhaseRecovered:          {models.PhaseCutoverInProgress},
}

// CanTransition checks if the job can advance from current to next phase.
func (e *Engine) CanTransition(current, next models.JobPhase) bool {
	allowed, ok := validTransitions[current]
	if !ok {
		return false
	}
	for _, p := range allowed {
		if p == next {
			return true
		}
	}
	return false
}

// Transition advances a job to the next phase after checking validity.
func (e *Engine) Transition(ctx context.Context, jobID string, nextPhase models.JobPhase, status models.JobStatus, lastErr string) (*models.UpgradeJob, error) {
	job, err := e.store.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if !e.CanTransition(job.CurrentPhase, nextPhase) && status != models.StatusFailed && status != models.StatusRecoverable {
		return nil, &apperrors.PhaseTransitionError{
			CurrentPhase: string(job.CurrentPhase),
			TargetPhase:  string(nextPhase),
			Reason:       fmt.Sprintf("illegal transition path from %s to %s", job.CurrentPhase, nextPhase),
		}
	}

	job.CurrentPhase = nextPhase
	job.Status = status
	job.LastError = lastErr
	job.UpdatedAt = time.Now().UTC()

	if err := e.store.UpdateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to update job state: %w", err)
	}

	return job, nil
}
