package store

import (
	"context"

	"eks-bluegreen-cli/pkg/models"
)

// Store defines operations for saving and retrieving upgrade workflow states.
type Store interface {
	CreateJob(ctx context.Context, job *models.UpgradeJob) error
	GetJob(ctx context.Context, id string) (*models.UpgradeJob, error)
	UpdateJob(ctx context.Context, job *models.UpgradeJob) error
	ListJobs(ctx context.Context) ([]*models.UpgradeJob, error)

	SaveResourcePlans(ctx context.Context, jobID string, plans []models.ResourcePlan) error
	GetResourcePlans(ctx context.Context, jobID string) ([]models.ResourcePlan, error)

	SaveApproval(ctx context.Context, approval *models.ApprovalRecord) error
	GetLatestApproval(ctx context.Context, jobID string) (*models.ApprovalRecord, error)

	SaveExecutionStep(ctx context.Context, step *models.ExecutionStep) error
	GetExecutionSteps(ctx context.Context, jobID string) ([]models.ExecutionStep, error)

	SaveValidationResults(ctx context.Context, jobID string, results []models.ValidationResult) error
	GetValidationResults(ctx context.Context, jobID string) ([]models.ValidationResult, error)

	Close() error
}
