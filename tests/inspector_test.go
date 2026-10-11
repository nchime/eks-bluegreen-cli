package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/deployer/k8s"
	"eks-bluegreen-cli/internal/inspector"
	"eks-bluegreen-cli/pkg/models"
)

// Test 3: Kubernetes Resource Inspection & Analysis Tests
func TestKubernetesResourceInspection(t *testing.T) {
	ctx := context.Background()
	awsMock := aws.NewMockAWSClient()
	k8sMock := k8s.NewMockK8sClient()

	ins := inspector.NewInspector(awsMock, k8sMock)
	result, err := ins.Inspect(ctx, "blue-cluster")
	require.NoError(t, err)

	// Cluster details
	assert.Equal(t, "blue-cluster", result.Cluster.Name)
	assert.Equal(t, "1.28", result.Cluster.Version)

	// Workloads
	assert.Contains(t, result.Workloads.Namespaces, "backend")
	assert.Len(t, result.Workloads.Deployments, 2)
	assert.Len(t, result.Workloads.StatefulSets, 1)
	assert.Len(t, result.Workloads.Services, 1)
	assert.Len(t, result.Workloads.Ingresses, 1)
	assert.Len(t, result.Workloads.PVCs, 1)

	// Deprecated API checks
	testWorkloads := &models.K8sWorkloadInfo{
		Deployments: []models.WorkloadItem{
			{Namespace: "legacy", Name: "old-api", APIVersion: "extensions/v1beta1"},
			{Namespace: "modern", Name: "new-api", APIVersion: "apps/v1"},
		},
		Ingresses: []models.IngressItem{
			{Namespace: "legacy", Name: "old-ing", Class: ""},
			{Namespace: "modern", Name: "new-ing", Class: "alb"},
		},
	}

	deprecations := inspector.CheckDeprecatedAPIs(testWorkloads, "1.28")
	require.Len(t, deprecations, 2)
	assert.Equal(t, "Deployment", deprecations[0].ResourceKind)
	assert.Equal(t, "1.16", deprecations[0].RemovedInVersion)
	assert.Equal(t, "Ingress", deprecations[1].ResourceKind)
}
