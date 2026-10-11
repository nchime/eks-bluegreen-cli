package tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"eks-bluegreen-cli/internal/approval"
	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/cutover"
	"eks-bluegreen-cli/internal/deployer/k8s"
	"eks-bluegreen-cli/internal/provisioner"
	"eks-bluegreen-cli/internal/recovery"
	"eks-bluegreen-cli/internal/store"
	"eks-bluegreen-cli/internal/validator"
	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

func setupTestJobAndStore(t *testing.T, jobID string) (store.Store, *models.UpgradeJob) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	st, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	job := &models.UpgradeJob{
		ID:            jobID,
		SourceCluster: "blue-cluster",
		SourceRegion:  "ap-northeast-2",
		TargetCluster: "green-cluster",
		TargetRegion:  "ap-northeast-2",
		SourceVersion: "1.28",
		TargetVersion: "1.30",
		CurrentPhase:  models.PhaseValidated,
		Status:        models.StatusCompleted,
		PlanHash:      "valid-plan-hash-111",
	}
	err = st.CreateJob(context.Background(), job)
	require.NoError(t, err)

	return st, job
}

// Test 6: Block cutover without explicit approval
func TestCutoverBlockedWithoutApproval(t *testing.T) {
	ctx := context.Background()
	st, job := setupTestJobAndStore(t, "job-cutover-no-app")
	defer st.Close()

	awsMock := aws.NewMockAWSClient()
	appMgr := approval.NewManager(st)
	op := cutover.NewOperator(awsMock, appMgr, st)

	// Cutover attempt before any approval record is saved
	err := op.ExecuteCutover(ctx, job.ID, "Z123456", "app.example.com", job.PlanHash, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrApprovalRequired, "Cutover must be strictly blocked if not approved")
}

// Test 7: Block cutover on validation failure
func TestCutoverBlockedOnValidationFailure(t *testing.T) {
	ctx := context.Background()
	st, job := setupTestJobAndStore(t, "job-cutover-val-fail")
	defer st.Close()

	// Save a failing validation result
	failedResults := []models.ValidationResult{
		{
			JobID:     job.ID,
			CheckName: "WorkloadPodReadiness",
			Severity:  models.SeverityCritical,
			Status:    models.ValidationFail,
			Details:   "0/3 pods ready",
			CheckedAt: time.Now().UTC(),
		},
	}
	err := st.SaveValidationResults(ctx, job.ID, failedResults)
	require.NoError(t, err)

	appMgr := approval.NewManager(st)

	// Attempting approval when critical validation failed must be rejected
	_, err = appMgr.Approve(ctx, job.ID, "sre-lead", "LGTM", job.PlanHash)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrValidationFailed, "Approval must be rejected when critical validations fail")

	// Cutover must also fail
	awsMock := aws.NewMockAWSClient()
	op := cutover.NewOperator(awsMock, appMgr, st)
	err = op.ExecuteCutover(ctx, job.ID, "Z123456", "app.example.com", job.PlanHash, "")
	require.Error(t, err)
}

// Test 8: Plan alteration invalidates existing approval
func TestPlanTamperingInvalidatesApproval(t *testing.T) {
	ctx := context.Background()
	st, job := setupTestJobAndStore(t, "job-plan-tamper")
	defer st.Close()

	// 1. Save passing validation
	passResults := []models.ValidationResult{
		{
			JobID:     job.ID,
			CheckName: "GreenClusterStatus",
			Severity:  models.SeverityInfo,
			Status:    models.ValidationPass,
			Details:   "Cluster active",
			CheckedAt: time.Now().UTC(),
		},
	}
	err := st.SaveValidationResults(ctx, job.ID, passResults)
	require.NoError(t, err)

	appMgr := approval.NewManager(st)

	// 2. Approve with original plan hash
	originalPlanHash := "valid-original-hash"
	_, err = appMgr.Approve(ctx, job.ID, "release-manager", "Approved", originalPlanHash)
	require.NoError(t, err)

	// 3. Plan modified! (PlanHash changed)
	modifiedPlanHash := "tampered-or-modified-hash-999"

	// 4. VerifyApproval fails due to hash mismatch
	err = appMgr.VerifyApproval(ctx, job.ID, modifiedPlanHash, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrApprovalInvalidated, "Approval must be invalidated if plan hash changed after approval")
}

// Test 11: Partial failure and recovery test
func TestPartialFailureAndRecovery(t *testing.T) {
	ctx := context.Background()
	st, job := setupTestJobAndStore(t, "job-failure-recovery")
	defer st.Close()

	awsMock := aws.NewMockAWSClient()
	rec := recovery.NewRecoverer(awsMock, st)

	// Traffic was pointing to green, now roll back to Blue
	err := rec.RecoverTraffic(ctx, job.ID, "Z123456", "app.example.com")
	require.NoError(t, err)

	// Verify DNS record in mock Route53 points back to Blue endpoint
	dnsVal, err := awsMock.GetDNSRecord(ctx, "Z123456", "app.example.com")
	require.NoError(t, err)
	blueCl, _ := awsMock.DescribeCluster(ctx, "blue-cluster")
	assert.Equal(t, blueCl.Endpoint, dnsVal, "DNS must be reverted to Blue cluster endpoint")

	// Job status updated to RECOVERED
	updatedJob, err := st.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.PhaseRecovered, updatedJob.CurrentPhase)
}

// Test 12: Dry-run does not mutate resources test
func TestDryRunDoesNotMutateResources(t *testing.T) {
	ctx := context.Background()
	st, job := setupTestJobAndStore(t, "job-dry-run-test")
	defer st.Close()

	awsMock := aws.NewMockAWSClient()
	prov := provisioner.NewProvisioner(awsMock, st)

	plan := &models.PlanReport{
		JobID:         job.ID,
		SourceCluster: "blue-cluster",
		TargetCluster: "green-cluster",
		ResourcePlans: []models.ResourcePlan{
			{ResourceType: "EKSControlPlane", ResourceName: "green-cluster"},
			{ResourceType: "NodeGroup", ResourceName: "ng-green"},
			{ResourceType: "EKSAddon", ResourceName: "vpc-cni"},
		},
	}

	// Execute with dryRun = true
	err := prov.ProvisionGreenInfrastructure(ctx, job, plan, true)
	require.NoError(t, err)

	// Verify zero mutation calls made to AWS mock
	calls := awsMock.GetCreatedCalls()
	assert.Empty(t, calls, "Dry-run must NOT execute any resource creation calls")
}

// Full Successful Workflow Integration Test (End-to-End lifecycle)
func TestEndToEndLifecycleWorkflow(t *testing.T) {
	ctx := context.Background()
	st, job := setupTestJobAndStore(t, "job-e2e-workflow")
	defer st.Close()

	awsMock := aws.NewMockAWSClient()
	k8sMock := k8s.NewMockK8sClient()

	// 0. Provision Green cluster
	prov := provisioner.NewProvisioner(awsMock, st)
	plan := &models.PlanReport{
		JobID:         job.ID,
		SourceCluster: "blue-cluster",
		TargetCluster: "green-cluster",
		TargetVersion: "1.30",
		ResourcePlans: []models.ResourcePlan{
			{ResourceType: "EKSControlPlane", ResourceName: "green-cluster"},
			{ResourceType: "NodeGroup", ResourceName: "ng-green"},
			{ResourceType: "EKSAddon", ResourceName: "vpc-cni"},
		},
	}
	err := prov.ProvisionGreenInfrastructure(ctx, job, plan, false)
	require.NoError(t, err)

	// 1. Validation passes
	v := validator.NewValidator(awsMock, k8sMock, st)
	valResults, valHash, err := v.Validate(ctx, job, nil, 5)
	require.NoError(t, err)
	assert.False(t, validator.HasCriticalFailures(valResults))
	assert.NotEmpty(t, valHash)

	// 2. Explicit Approval
	appMgr := approval.NewManager(st)
	appRec, err := appMgr.Approve(ctx, job.ID, "lead-architect", "Validation passed cleanly", job.PlanHash)
	require.NoError(t, err)
	assert.Equal(t, models.ApprovalStatusApproved, appRec.Status)

	// 3. Cutover succeeds
	op := cutover.NewOperator(awsMock, appMgr, st)
	err = op.ExecuteCutover(ctx, job.ID, "Z123456", "app.example.com", job.PlanHash, valHash)
	require.NoError(t, err)

	// 4. Verify Traffic switched to Green
	dnsVal, err := awsMock.GetDNSRecord(ctx, "Z123456", "app.example.com")
	require.NoError(t, err)
	assert.Contains(t, dnsVal, "green-cluster")

	// 5. Verify Blue cluster was never deleted
	blueInfo, err := awsMock.DescribeCluster(ctx, "blue-cluster")
	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", blueInfo.Status)
}
