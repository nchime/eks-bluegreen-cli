package aws

import (
	"context"

	"eks-bluegreen-cli/pkg/models"
)

// CreateClusterParams holds inputs for Green cluster creation.
type CreateClusterParams struct {
	Name            string
	Version         string
	RoleArn         string
	SubnetIDs       []string
	SecurityGroupIDs []string
	Tags            map[string]string
}

// Client abstracts AWS SDK calls for testability.
type Client interface {
	CheckCredentials(ctx context.Context) (accountID string, arn string, err error)
	DescribeCluster(ctx context.Context, clusterName string) (*models.EKSClusterInfo, error)
	ListClusters(ctx context.Context) ([]string, error)
	ListNodeGroups(ctx context.Context, clusterName string) ([]models.NodeGroupInfo, error)
	ListAddons(ctx context.Context, clusterName string) ([]models.AddonInfo, error)
	DescribeAddonVersions(ctx context.Context, kubernetesVersion string) ([]models.AddonCompatibilityCheck, error)
	DescribeVpc(ctx context.Context, vpcID string) (*models.NetworkInfo, error)
	CreateCluster(ctx context.Context, params CreateClusterParams) (*models.EKSClusterInfo, error)
	CreateNodeGroup(ctx context.Context, clusterName string, ng models.NodeGroupInfo) error
	CreateAddon(ctx context.Context, clusterName, addonName, version string) error
	UpdateDNSRecord(ctx context.Context, zoneID, recordName, targetEndpoint string) error
	GetDNSRecord(ctx context.Context, zoneID, recordName string) (string, error)
}
