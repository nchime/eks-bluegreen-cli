package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/deployer/k8s"
	"eks-bluegreen-cli/internal/inspector"
	"eks-bluegreen-cli/internal/planner"
	"eks-bluegreen-cli/pkg/models"
)

// Test 4: Plan Generation and Compatibility Check Tests
func TestPlanGenerationAndCompatibility(t *testing.T) {
	ctx := context.Background()
	awsMock := aws.NewMockAWSClient()
	k8sMock := k8s.NewMockK8sClient()

	ins := inspector.NewInspector(awsMock, k8sMock)
	inspection, err := ins.Inspect(ctx, "blue-cluster")
	require.NoError(t, err)

	p := planner.NewPlanner(awsMock)

	// 1. Valid upgrade plan (1.28 -> 1.30)
	plan, err := p.GeneratePlan(ctx, "job-test-1", inspection, "green-cluster", "1.30", "ap-northeast-2")
	require.NoError(t, err)
	assert.True(t, plan.Compatible)
	assert.Empty(t, plan.BlockingReasons)
	assert.NotEmpty(t, plan.PlanHash)
	assert.Greater(t, plan.EstimatedCost.EstimatedTotalMonthlyUSD, 0.0)

	// Check classified resources
	var hasControlPlane, hasNG, hasStatefulSet, hasPVC bool
	for _, rp := range plan.ResourcePlans {
		if rp.ResourceType == "EKSControlPlane" {
			hasControlPlane = true
			assert.Equal(t, models.ActionReplicate, rp.Action)
		}
		if rp.ResourceType == "NodeGroup" {
			hasNG = true
		}
		if rp.ResourceType == "StatefulSet" {
			hasStatefulSet = true
			assert.Equal(t, models.ActionManualCheck, rp.Action)
			assert.Equal(t, models.RiskHigh, rp.RiskLevel)
		}
		if rp.ResourceType == "PersistentVolumeClaim" {
			hasPVC = true
			assert.Equal(t, models.ActionManualCheck, rp.Action)
			assert.Equal(t, models.RiskCritical, rp.RiskLevel)
		}
	}
	assert.True(t, hasControlPlane)
	assert.True(t, hasNG)
	assert.True(t, hasStatefulSet)
	assert.True(t, hasPVC)

	// 2. Unsupported version test
	invalidPlan, err := p.GeneratePlan(ctx, "job-test-2", inspection, "green-cluster", "1.99", "ap-northeast-2")
	require.NoError(t, err)
	assert.False(t, invalidPlan.Compatible)
	assert.Contains(t, invalidPlan.BlockingReasons[0], "not in recognized supported EKS versions")

	// 3. Version downgrade test (target 1.28 <= source 1.28)
	downgradePlan, err := p.GeneratePlan(ctx, "job-test-3", inspection, "green-cluster", "1.28", "ap-northeast-2")
	require.NoError(t, err)
	assert.False(t, downgradePlan.Compatible)
	assert.Contains(t, downgradePlan.BlockingReasons[0], "must be higher than current source version")
}
