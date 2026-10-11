package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"eks-bluegreen-cli/internal/security"
	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

// Test 10: Secret Masking Test
func TestSecretMasking(t *testing.T) {
	rawLogs := `
Connecting to AWS with aws_access_key_id: AKIAIOSFODNN7EXAMPLE and aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.t-IDcSemACt8x4iTMC6Y5iVOnkOWW0WSC shovel
Database config: "password": "supersecretpassword123", "client_secret": "myoauthsecret456"
Kubernetes secret: password: superSecretValue999
`

	masked := security.MaskSensitiveData(rawLogs)

	assert.NotContains(t, masked, "AKIAIOSFODNN7EXAMPLE")
	assert.NotContains(t, masked, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	assert.NotContains(t, masked, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	assert.NotContains(t, masked, "supersecretpassword123")
	assert.NotContains(t, masked, "myoauthsecret456")
	assert.NotContains(t, masked, "superSecretValue999")

	assert.Contains(t, masked, "[REDACTED]")
}

// Test 9: Blue Cluster Deletion and Modification Prevention Test
func TestBlueClusterProtectionGuardrails(t *testing.T) {
	blueCluster := "prod-blue-cluster"

	// 1. Target equals Blue cluster during provisioning
	err := security.AssertSafeOperation("prod-blue-cluster", blueCluster, "provisioning")
	require.Error(t, err)
	var guardErr *apperrors.GuardrailError
	assert.ErrorAs(t, err, &guardErr)
	assert.Contains(t, guardErr.Reason, "target cluster cannot be identical to source")

	// 2. Target equals Blue cluster with case insensitivity
	err = security.AssertSafeOperation("PROD-BLUE-CLUSTER", blueCluster, "provisioning")
	require.Error(t, err)

	// 3. Destructive action on Blue cluster
	err = security.ValidateBlueClusterProtection("prod-blue-cluster", blueCluster)
	assert.ErrorIs(t, err, apperrors.ErrBlueClusterDeletion)

	// 4. Safe Green cluster operation allowed
	err = security.AssertSafeOperation("prod-green-cluster", blueCluster, "provisioning")
	assert.NoError(t, err)
}

// Deterministic Plan Hash Test
func TestDeterministicPlanHash(t *testing.T) {
	p1 := &models.PlanReport{
		SourceCluster: "blue",
		TargetCluster: "green",
		ResourcePlans: []models.ResourcePlan{
			{ResourceType: "Deployment", ResourceName: "app"},
			{ResourceType: "Service", ResourceName: "svc"},
		},
	}

	p2 := &models.PlanReport{
		SourceCluster: "blue",
		TargetCluster: "green",
		ResourcePlans: []models.ResourcePlan{
			{ResourceType: "Service", ResourceName: "svc"},
			{ResourceType: "Deployment", ResourceName: "app"},
		},
	}

	hash1, err1 := security.ComputePlanHash(p1)
	hash2, err2 := security.ComputePlanHash(p2)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEmpty(t, hash1)
	assert.Equal(t, hash1, hash2, "Plan hash must be order-independent and deterministic")
}
