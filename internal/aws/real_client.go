package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"eks-bluegreen-cli/pkg/models"
)

type RealAWSClient struct {
	cfg     aws.Config
	eks     *eks.Client
	ec2     *ec2.Client
	sts     *sts.Client
	route53 *route53.Client
	region  string
}

// NewRealAWSClient initializes AWS SDK v2 clients.
func NewRealAWSClient(ctx context.Context, region, profile string) (*RealAWSClient, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	return &RealAWSClient{
		cfg:     cfg,
		eks:     eks.NewFromConfig(cfg),
		ec2:     ec2.NewFromConfig(cfg),
		sts:     sts.NewFromConfig(cfg),
		route53: route53.NewFromConfig(cfg),
		region:  cfg.Region,
	}, nil
}

func (c *RealAWSClient) CheckCredentials(ctx context.Context) (string, string, error) {
	out, err := c.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", "", fmt.Errorf("failed to authenticate with AWS: %w", err)
	}
	return aws.ToString(out.Account), aws.ToString(out.Arn), nil
}

func (c *RealAWSClient) DescribeCluster(ctx context.Context, clusterName string) (*models.EKSClusterInfo, error) {
	out, err := c.eks.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster %q: %w", clusterName, err)
	}

	cl := out.Cluster
	if cl == nil {
		return nil, fmt.Errorf("cluster %q not found", clusterName)
	}

	var oidc string
	if cl.Identity != nil && cl.Identity.Oidc != nil {
		oidc = aws.ToString(cl.Identity.Oidc.Issuer)
	}

	var vpcID string
	var subnets []string
	var secGroups []string
	if cl.ResourcesVpcConfig != nil {
		vpcID = aws.ToString(cl.ResourcesVpcConfig.VpcId)
		subnets = cl.ResourcesVpcConfig.SubnetIds
		secGroups = cl.ResourcesVpcConfig.SecurityGroupIds
	}

	return &models.EKSClusterInfo{
		Name:             aws.ToString(cl.Name),
		Arn:              aws.ToString(cl.Arn),
		Version:          aws.ToString(cl.Version),
		Endpoint:         aws.ToString(cl.Endpoint),
		RoleArn:          aws.ToString(cl.RoleArn),
		Status:           string(cl.Status),
		VpcID:            vpcID,
		SubnetIDs:        subnets,
		SecurityGroupIDs: secGroups,
		OIDCProvider:     oidc,
		Tags:             cl.Tags,
	}, nil
}

func (c *RealAWSClient) ListClusters(ctx context.Context) ([]string, error) {
	out, err := c.eks.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, err
	}
	return out.Clusters, nil
}

func (c *RealAWSClient) ListNodeGroups(ctx context.Context, clusterName string) ([]models.NodeGroupInfo, error) {
	listOut, err := c.eks.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list node groups for %q: %w", clusterName, err)
	}

	var results []models.NodeGroupInfo
	for _, ngName := range listOut.Nodegroups {
		desc, err := c.eks.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(ngName),
		})
		if err != nil {
			return nil, err
		}

		ng := desc.Nodegroup
		if ng == nil {
			continue
		}

		info := models.NodeGroupInfo{
			Name:           aws.ToString(ng.NodegroupName),
			ClusterName:    clusterName,
			Status:         string(ng.Status),
			CapacityType:   string(ng.CapacityType),
			InstanceTypes:  ng.InstanceTypes,
			Subnets:        ng.Subnets,
			NodeRole:       aws.ToString(ng.NodeRole),
			AmiType:        string(ng.AmiType),
			ReleaseVersion: aws.ToString(ng.ReleaseVersion),
			Labels:         ng.Labels,
		}
		if ng.ScalingConfig != nil {
			info.DesiredSize = aws.ToInt32(ng.ScalingConfig.DesiredSize)
			info.MinSize = aws.ToInt32(ng.ScalingConfig.MinSize)
			info.MaxSize = aws.ToInt32(ng.ScalingConfig.MaxSize)
		}
		results = append(results, info)
	}
	return results, nil
}

func (c *RealAWSClient) ListAddons(ctx context.Context, clusterName string) ([]models.AddonInfo, error) {
	listOut, err := c.eks.ListAddons(ctx, &eks.ListAddonsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list addons for %q: %w", clusterName, err)
	}

	var addons []models.AddonInfo
	for _, addonName := range listOut.Addons {
		desc, err := c.eks.DescribeAddon(ctx, &eks.DescribeAddonInput{
			ClusterName: aws.String(clusterName),
			AddonName:   aws.String(addonName),
		})
		if err != nil {
			return nil, err
		}
		a := desc.Addon
		if a == nil {
			continue
		}
		addons = append(addons, models.AddonInfo{
			Name:              aws.ToString(a.AddonName),
			Version:           aws.ToString(a.AddonVersion),
			Status:            string(a.Status),
			ServiceAccountArn: aws.ToString(a.ServiceAccountRoleArn),
		})
	}
	return addons, nil
}

func (c *RealAWSClient) DescribeAddonVersions(ctx context.Context, kubernetesVersion string) ([]models.AddonCompatibilityCheck, error) {
	out, err := c.eks.DescribeAddonVersions(ctx, &eks.DescribeAddonVersionsInput{
		KubernetesVersion: aws.String(kubernetesVersion),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe addon versions for %q: %w", kubernetesVersion, err)
	}

	var checks []models.AddonCompatibilityCheck
	for _, a := range out.Addons {
		name := aws.ToString(a.AddonName)
		var targetVer string
		if len(a.AddonVersions) > 0 {
			targetVer = aws.ToString(a.AddonVersions[0].AddonVersion)
		}
		checks = append(checks, models.AddonCompatibilityCheck{
			Name:           name,
			TargetVersion:  targetVer,
			IsCompatible:   true,
			Recommendation: fmt.Sprintf("Use compatible addon version %s for EKS %s", targetVer, kubernetesVersion),
		})
	}
	return checks, nil
}

func (c *RealAWSClient) DescribeVpc(ctx context.Context, vpcID string) (*models.NetworkInfo, error) {
	vpcOut, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	if err != nil || len(vpcOut.Vpcs) == 0 {
		return nil, fmt.Errorf("failed to describe vpc %q: %w", vpcID, err)
	}

	cidr := aws.ToString(vpcOut.Vpcs[0].CidrBlock)

	subOut, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets for vpc %q: %w", vpcID, err)
	}

	var subnets []models.SubnetDetail
	for _, s := range subOut.Subnets {
		subnets = append(subnets, models.SubnetDetail{
			SubnetID:         aws.ToString(s.SubnetId),
			AvailabilityZone: aws.ToString(s.AvailabilityZone),
			CidrBlock:        aws.ToString(s.CidrBlock),
			AvailableIPs:     aws.ToInt32(s.AvailableIpAddressCount),
			IsPublic:         aws.ToBool(s.MapPublicIpOnLaunch),
		})
	}

	sgOut, err := c.ec2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe security groups: %w", err)
	}

	var sgs []models.SecurityGroupDetail
	for _, sg := range sgOut.SecurityGroups {
		sgs = append(sgs, models.SecurityGroupDetail{
			GroupID:     aws.ToString(sg.GroupId),
			GroupName:   aws.ToString(sg.GroupName),
			Description: aws.ToString(sg.Description),
		})
	}

	return &models.NetworkInfo{
		VpcID:     vpcID,
		CidrBlock: cidr,
		Subnets:   subnets,
		SecGroups: sgs,
	}, nil
}

func (c *RealAWSClient) CreateCluster(ctx context.Context, params CreateClusterParams) (*models.EKSClusterInfo, error) {
	out, err := c.eks.CreateCluster(ctx, &eks.CreateClusterInput{
		Name:    aws.String(params.Name),
		Version: aws.String(params.Version),
		RoleArn: aws.String(params.RoleArn),
		ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
			SubnetIds:        params.SubnetIDs,
			SecurityGroupIds: params.SecurityGroupIDs,
		},
		Tags: params.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster %q: %w", params.Name, err)
	}

	return &models.EKSClusterInfo{
		Name:      aws.ToString(out.Cluster.Name),
		Arn:       aws.ToString(out.Cluster.Arn),
		Version:   aws.ToString(out.Cluster.Version),
		Endpoint:  aws.ToString(out.Cluster.Endpoint),
		RoleArn:   aws.ToString(out.Cluster.RoleArn),
		Status:    string(out.Cluster.Status),
		VpcID:     aws.ToString(out.Cluster.ResourcesVpcConfig.VpcId),
		SubnetIDs: out.Cluster.ResourcesVpcConfig.SubnetIds,
	}, nil
}

func (c *RealAWSClient) CreateNodeGroup(ctx context.Context, clusterName string, ng models.NodeGroupInfo) error {
	_, err := c.eks.CreateNodegroup(ctx, &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(ng.Name),
		NodeRole:      aws.String(ng.NodeRole),
		Subnets:       ng.Subnets,
		InstanceTypes: ng.InstanceTypes,
		AmiType:       ekstypes.AMITypes(ng.AmiType),
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: aws.Int32(ng.DesiredSize),
			MinSize:     aws.Int32(ng.MinSize),
			MaxSize:     aws.Int32(ng.MaxSize),
		},
		Labels: ng.Labels,
	})
	if err != nil {
		return fmt.Errorf("failed to create nodegroup %q: %w", ng.Name, err)
	}
	return nil
}

func (c *RealAWSClient) CreateAddon(ctx context.Context, clusterName, addonName, version string) error {
	input := &eks.CreateAddonInput{
		ClusterName: aws.String(clusterName),
		AddonName:   aws.String(addonName),
	}
	if version != "" {
		input.AddonVersion = aws.String(version)
	}
	_, err := c.eks.CreateAddon(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create addon %q: %w", addonName, err)
	}
	return nil
}

func (c *RealAWSClient) UpdateDNSRecord(ctx context.Context, zoneID, recordName, targetEndpoint string) error {
	_, err := c.route53.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &route53types.ChangeBatch{
			Comment: aws.String("EKS Blue-Green upgrade cutover update"),
			Changes: []route53types.Change{
				{
					Action: route53types.ChangeActionUpsert,
					ResourceRecordSet: &route53types.ResourceRecordSet{
						Name: aws.String(recordName),
						Type: route53types.RRTypeCname,
						TTL:  aws.Int64(60),
						ResourceRecords: []route53types.ResourceRecord{
							{Value: aws.String(targetEndpoint)},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update route53 record: %w", err)
	}
	return nil
}

func (c *RealAWSClient) GetDNSRecord(ctx context.Context, zoneID, recordName string) (string, error) {
	out, err := c.route53.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(zoneID),
		StartRecordName: aws.String(recordName),
		MaxItems:        aws.Int32(1),
	})
	if err != nil {
		return "", err
	}
	for _, r := range out.ResourceRecordSets {
		if strings.TrimSuffix(aws.ToString(r.Name), ".") == strings.TrimSuffix(recordName, ".") {
			if len(r.ResourceRecords) > 0 {
				return aws.ToString(r.ResourceRecords[0].Value), nil
			}
		}
	}
	return "", fmt.Errorf("record %q not found", recordName)
}
