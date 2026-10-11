package tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"eks-bluegreen-cli/internal/store"
	"eks-bluegreen-cli/pkg/models"
)

// Test 5: Idempotency and Job Resume Tests
func TestStoreIdempotencyAndJobResume(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st.Close()

	// 1. Create Job
	job := &models.UpgradeJob{
		ID:            "job-resume-101",
		SourceCluster: "blue-cluster",
		SourceRegion:  "ap-northeast-2",
		TargetCluster: "green-cluster",
		TargetRegion:  "ap-northeast-2",
		SourceVersion: "1.28",
		TargetVersion: "1.30",
		CurrentPhase:  models.PhaseInit,
		Status:        models.StatusPending,
		PlanPath:      "./plans/upgrade.yaml",
		PlanHash:      "hash-123456",
	}

	err = st.CreateJob(ctx, job)
	require.NoError(t, err)

	// Retrieve job
	loaded, err := st.GetJob(ctx, "job-resume-101")
	require.NoError(t, err)
	assert.Equal(t, models.PhaseInit, loaded.CurrentPhase)
	assert.Equal(t, models.StatusPending, loaded.Status)

	// 2. Partial execution step
	step1 := &models.ExecutionStep{
		JobID:     "job-resume-101",
		StepName:  "CreateEKSControlPlane",
		Status:    models.StepStatusCompleted,
		StartedAt: time.Now().UTC(),
	}
	err = st.SaveExecutionStep(ctx, step1)
	require.NoError(t, err)

	// Simulate failure on step 2
	step2 := &models.ExecutionStep{
		JobID:        "job-resume-101",
		StepName:     "CreateNodeGroup",
		Status:       models.StepStatusFailed,
		StartedAt:    time.Now().UTC(),
		ErrorMessage: "Insufficient subnet IPs",
	}
	err = st.SaveExecutionStep(ctx, step2)
	require.NoError(t, err)

	// Update job to FAILED/RECOVERABLE
	loaded.CurrentPhase = models.PhaseProvisioning
	loaded.Status = models.StatusRecoverable
	loaded.LastError = "Insufficient subnet IPs"
	err = st.UpdateJob(ctx, loaded)
	require.NoError(t, err)

	// 3. Resume job: Verify state can be read from store after restart
	st2, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer st2.Close()

	resumedJob, err := st2.GetJob(ctx, "job-resume-101")
	require.NoError(t, err)
	assert.Equal(t, models.PhaseProvisioning, resumedJob.CurrentPhase)
	assert.Equal(t, models.StatusRecoverable, resumedJob.Status)
	assert.Equal(t, "Insufficient subnet IPs", resumedJob.LastError)

	steps, err := st2.GetExecutionSteps(ctx, "job-resume-101")
	require.NoError(t, err)
	assert.Len(t, steps, 2)
	assert.Equal(t, models.StepStatusCompleted, steps[0].Status)
	assert.Equal(t, models.StepStatusFailed, steps[1].Status)

	// Retry / Resume step 2 to completed
	now := time.Now().UTC()
	step2Retried := &models.ExecutionStep{
		JobID:       "job-resume-101",
		StepName:    "CreateNodeGroup",
		Status:      models.StepStatusCompleted,
		StartedAt:   time.Now().UTC(),
		CompletedAt: &now,
		RetryCount:  1,
	}
	err = st2.SaveExecutionStep(ctx, step2Retried)
	require.NoError(t, err)

	resumedJob.Status = models.StatusCompleted
	resumedJob.CurrentPhase = models.PhaseProvisioned
	resumedJob.LastError = ""
	err = st2.UpdateJob(ctx, resumedJob)
	require.NoError(t, err)

	finalJob, err := st2.GetJob(ctx, "job-resume-101")
	require.NoError(t, err)
	assert.Equal(t, models.PhaseProvisioned, finalJob.CurrentPhase)
	assert.Equal(t, models.StatusCompleted, finalJob.Status)
}
