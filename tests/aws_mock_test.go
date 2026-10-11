package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"eks-bluegreen-cli/internal/aws"
	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

// Test 2: AWS API Mock Tests
func TestAWSMockOperations(t *testing.T) {
	ctx := context.Background()
	mock := aws.NewMockAWSClient()

	// 1. Credentials
	acc, arn, err := mock.CheckCredentials(ctx)
	require.NoError(t, err)
	assert.Equal(t, "123456789012", acc)
	assert.Contains(t, arn, "arn:aws:iam")

	// 2. Describe Cluster
	cl, err := mock.DescribeCluster(ctx, "blue-cluster")
	require.NoError(t, err)
	assert.Equal(t, "blue-cluster", cl.Name)
	assert.Equal(t, "1.28", cl.Version)
	assert.Equal(t, "ACTIVE", cl.Status)
	assert.Equal(t, "vpc-12345", cl.VpcID)

	// 3. Describe non-existent cluster
	_, err = mock.DescribeCluster(ctx, "non-existent")
	assert.Error(t, err)

	// 4. NodeGroups
	ngs, err := mock.ListNodeGroups(ctx, "blue-cluster")
	require.NoError(t, err)
	require.Len(t, ngs, 1)
	assert.Equal(t, "ng-general", ngs[0].Name)
	assert.Equal(t, int32(3), ngs[0].DesiredSize)

	// 5. Addons
	addons, err := mock.ListAddons(ctx, "blue-cluster")
	require.NoError(t, err)
	assert.Len(t, addons, 3)

	// 6. Describe VPC
	net, err := mock.DescribeVpc(ctx, "vpc-12345")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/16", net.CidrBlock)
	assert.Len(t, net.Subnets, 3)

	// 7. Create Cluster
	created, err := mock.CreateCluster(ctx, aws.CreateClusterParams{
		Name:             "green-cluster",
		Version:          "1.30",
		RoleArn:          cl.RoleArn,
		SubnetIDs:        cl.SubnetIDs,
		SecurityGroupIDs: cl.SecurityGroupIDs,
	})
	require.NoError(t, err)
	assert.Equal(t, "green-cluster", created.Name)
	assert.Equal(t, "1.30", created.Version)

	// 8. Create Node Group
	err = mock.CreateNodeGroup(ctx, "green-cluster", models.NodeGroupInfo{
		Name:          "ng-green-1",
		InstanceTypes: []string{"m5.large"},
		DesiredSize:   3,
	})
	require.NoError(t, err)

	greenNGs, err := mock.ListNodeGroups(ctx, "green-cluster")
	require.NoError(t, err)
	assert.Len(t, greenNGs, 1)

	// 9. DNS Records
	err = mock.UpdateDNSRecord(ctx, "Z123456", "app.example.com", "green-alb.endpoint.com")
	require.NoError(t, err)

	recVal, err := mock.GetDNSRecord(ctx, "Z123456", "app.example.com")
	require.NoError(t, err)
	assert.Equal(t, "green-alb.endpoint.com", recVal)

	// 10. Auth failure simulation
	mock.AuthFail = true
	_, _, err = mock.CheckCredentials(ctx)
	assert.ErrorIs(t, err, apperrors.ErrAWSAuthFailed)
}
