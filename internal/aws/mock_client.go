package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

type MockAWSClient struct {
	mu            sync.RWMutex
	AccountID     string
	Arn           string
	Clusters      map[string]*models.EKSClusterInfo
	NodeGroups    map[string][]models.NodeGroupInfo // clusterName -> node groups
	Addons        map[string][]models.AddonInfo     // clusterName -> addons
	Networks      map[string]*models.NetworkInfo    // vpcId -> network info
	DNSRecords    map[string]map[string]string      // zoneId -> (recordName -> value)
	CreatedCalls   []string                          // tracks mutation operations for DryRun testing
	AuthFail       bool
	SimulateError  error
	persistentFile string
}

func NewMockAWSClient() *MockAWSClient {
	m := &MockAWSClient{
		AccountID:    "123456789012",
		Arn:          "arn:aws:iam::123456789012:user/admin",
		Clusters:     make(map[string]*models.EKSClusterInfo),
		NodeGroups:   make(map[string][]models.NodeGroupInfo),
		Addons:       make(map[string][]models.AddonInfo),
		Networks:     make(map[string]*models.NetworkInfo),
		DNSRecords:   make(map[string]map[string]string),
		CreatedCalls: []string{},
	}

	// Seed standard VPC
	m.Networks["vpc-12345"] = &models.NetworkInfo{
		VpcID:     "vpc-12345",
		CidrBlock: "10.0.0.0/16",
		Subnets: []models.SubnetDetail{
			{SubnetID: "subnet-a1", AvailabilityZone: "ap-northeast-2a", CidrBlock: "10.0.1.0/24", AvailableIPs: 250, IsPublic: false},
			{SubnetID: "subnet-b1", AvailabilityZone: "ap-northeast-2b", CidrBlock: "10.0.2.0/24", AvailableIPs: 250, IsPublic: false},
			{SubnetID: "subnet-pub", AvailabilityZone: "ap-northeast-2a", CidrBlock: "10.0.10.0/24", AvailableIPs: 245, IsPublic: true},
		},
		SecGroups: []models.SecurityGroupDetail{
			{GroupID: "sg-cluster", GroupName: "eks-cluster-sg", Description: "EKS control plane security group"},
		},
	}

	// Seed sample blue cluster
	m.Clusters["blue-cluster"] = &models.EKSClusterInfo{
		Name:             "blue-cluster",
		Arn:              "arn:aws:eks:ap-northeast-2:123456789012:cluster/blue-cluster",
		Version:          "1.28",
		Endpoint:         "https://blue-eks.ap-northeast-2.eks.amazonaws.com",
		RoleArn:          "arn:aws:iam::123456789012:role/eks-cluster-role",
		Status:           "ACTIVE",
		VpcID:            "vpc-12345",
		SubnetIDs:        []string{"subnet-a1", "subnet-b1"},
		SecurityGroupIDs: []string{"sg-cluster"},
		OIDCProvider:     "https://oidc.eks.ap-northeast-2.amazonaws.com/id/EXAMPLED3B45F6789012",
		Tags: map[string]string{
			"Environment": "Production",
			"ManagedBy":   "Terraform",
		},
	}

	m.NodeGroups["blue-cluster"] = []models.NodeGroupInfo{
		{
			Name:           "ng-general",
			ClusterName:    "blue-cluster",
			Status:         "ACTIVE",
			CapacityType:   "ON_DEMAND",
			InstanceTypes:  []string{"m5.large"},
			DesiredSize:    3,
			MinSize:        2,
			MaxSize:        5,
			Subnets:        []string{"subnet-a1", "subnet-b1"},
			NodeRole:       "arn:aws:iam::123456789012:role/eks-nodegroup-role",
			AmiType:        "AL2_x86_64",
			ReleaseVersion: "1.28.2-20231027",
			Labels: map[string]string{
				"role": "general",
			},
		},
	}

	m.Addons["blue-cluster"] = []models.AddonInfo{
		{Name: "vpc-cni", Version: "v1.15.1-eksbuild.1", Status: "ACTIVE"},
		{Name: "kube-proxy", Version: "v1.28.2-eksbuild.2", Status: "ACTIVE"},
		{Name: "coredns", Version: "v1.10.1-eksbuild.4", Status: "ACTIVE"},
	}

	// Seed DNS
	m.DNSRecords["Z123456"] = map[string]string{
		"app.example.com": "blue-alb-12345.ap-northeast-2.elb.amazonaws.com",
	}

	return m
}

func (m *MockAWSClient) EnableFilePersistence(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.persistentFile = path
	m.loadFromFile()
}

type mockPersistentState struct {
	Clusters   map[string]*models.EKSClusterInfo `json:"clusters"`
	NodeGroups map[string][]models.NodeGroupInfo `json:"node_groups"`
	Addons     map[string][]models.AddonInfo     `json:"addons"`
	DNSRecords map[string]map[string]string      `json:"dns_records"`
}

func (m *MockAWSClient) loadFromFile() {
	if m.persistentFile == "" {
		return
	}
	data, err := os.ReadFile(m.persistentFile)
	if err != nil {
		return
	}
	var state mockPersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	for k, v := range state.Clusters {
		m.Clusters[k] = v
	}
	for k, v := range state.NodeGroups {
		m.NodeGroups[k] = v
	}
	for k, v := range state.Addons {
		m.Addons[k] = v
	}
	for k, v := range state.DNSRecords {
		m.DNSRecords[k] = v
	}
}

func (m *MockAWSClient) saveToFile() {
	if m.persistentFile == "" {
		return
	}
	state := mockPersistentState{
		Clusters:   m.Clusters,
		NodeGroups: m.NodeGroups,
		Addons:     m.Addons,
		DNSRecords: m.DNSRecords,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll("./.eksbg", 0755)
	_ = os.WriteFile(m.persistentFile, data, 0644)
}

func (m *MockAWSClient) CheckCredentials(ctx context.Context) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.AuthFail {
		return "", "", apperrors.ErrAWSAuthFailed
	}
	return m.AccountID, m.Arn, nil
}

func (m *MockAWSClient) DescribeCluster(ctx context.Context, clusterName string) (*models.EKSClusterInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.SimulateError != nil {
		return nil, m.SimulateError
	}
	cl, exists := m.Clusters[clusterName]
	if !exists {
		return nil, fmt.Errorf("ResourceNotFoundException: cluster %q not found", clusterName)
	}
	// Return a copy
	res := *cl
	return &res, nil
}

func (m *MockAWSClient) ListClusters(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []string
	for k := range m.Clusters {
		list = append(list, k)
	}
	return list, nil
}

func (m *MockAWSClient) ListNodeGroups(ctx context.Context, clusterName string) ([]models.NodeGroupInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ngs, exists := m.NodeGroups[clusterName]
	if !exists {
		return []models.NodeGroupInfo{}, nil
	}
	res := make([]models.NodeGroupInfo, len(ngs))
	copy(res, ngs)
	return res, nil
}

func (m *MockAWSClient) ListAddons(ctx context.Context, clusterName string) ([]models.AddonInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	addons, exists := m.Addons[clusterName]
	if !exists {
		return []models.AddonInfo{}, nil
	}
	res := make([]models.AddonInfo, len(addons))
	copy(res, addons)
	return res, nil
}

func (m *MockAWSClient) DescribeAddonVersions(ctx context.Context, kubernetesVersion string) ([]models.AddonCompatibilityCheck, error) {
	return []models.AddonCompatibilityCheck{
		{
			Name:           "vpc-cni",
			CurrentVersion: "v1.15.1-eksbuild.1",
			TargetVersion:  "v1.18.1-eksbuild.3",
			IsCompatible:   true,
			Recommendation: "Upgrade vpc-cni to v1.18.1-eksbuild.3 for Kubernetes " + kubernetesVersion,
		},
		{
			Name:           "kube-proxy",
			CurrentVersion: "v1.28.2-eksbuild.2",
			TargetVersion:  "v1.30.0-eksbuild.1",
			IsCompatible:   true,
			Recommendation: "Upgrade kube-proxy to v1.30.0-eksbuild.1 for Kubernetes " + kubernetesVersion,
		},
		{
			Name:           "coredns",
			CurrentVersion: "v1.10.1-eksbuild.4",
			TargetVersion:  "v1.11.1-eksbuild.4",
			IsCompatible:   true,
			Recommendation: "Upgrade coredns to v1.11.1-eksbuild.4 for Kubernetes " + kubernetesVersion,
		},
	}, nil
}

func (m *MockAWSClient) DescribeVpc(ctx context.Context, vpcID string) (*models.NetworkInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	net, exists := m.Networks[vpcID]
	if !exists {
		return nil, fmt.Errorf("VPC %q not found", vpcID)
	}
	res := *net
	return &res, nil
}

func (m *MockAWSClient) CreateCluster(ctx context.Context, params CreateClusterParams) (*models.EKSClusterInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := fmt.Sprintf("CreateCluster:%s:%s", params.Name, params.Version)
	m.CreatedCalls = append(m.CreatedCalls, call)

	cl := &models.EKSClusterInfo{
		Name:             params.Name,
		Arn:              fmt.Sprintf("arn:aws:eks:ap-northeast-2:123456789012:cluster/%s", params.Name),
		Version:          params.Version,
		Endpoint:         fmt.Sprintf("https://%s.ap-northeast-2.eks.amazonaws.com", params.Name),
		RoleArn:          params.RoleArn,
		Status:           "ACTIVE",
		SubnetIDs:        params.SubnetIDs,
		SecurityGroupIDs: params.SecurityGroupIDs,
		OIDCProvider:     fmt.Sprintf("https://oidc.eks.ap-northeast-2.amazonaws.com/id/%s-oidc", params.Name),
		Tags:             params.Tags,
	}
	m.Clusters[params.Name] = cl
	m.saveToFile()
	return cl, nil
}

func (m *MockAWSClient) CreateNodeGroup(ctx context.Context, clusterName string, ng models.NodeGroupInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := fmt.Sprintf("CreateNodeGroup:%s:%s", clusterName, ng.Name)
	m.CreatedCalls = append(m.CreatedCalls, call)

	ng.ClusterName = clusterName
	ng.Status = "ACTIVE"
	m.NodeGroups[clusterName] = append(m.NodeGroups[clusterName], ng)
	m.saveToFile()
	return nil
}

func (m *MockAWSClient) CreateAddon(ctx context.Context, clusterName, addonName, version string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := fmt.Sprintf("CreateAddon:%s:%s:%s", clusterName, addonName, version)
	m.CreatedCalls = append(m.CreatedCalls, call)

	m.Addons[clusterName] = append(m.Addons[clusterName], models.AddonInfo{
		Name:    addonName,
		Version: version,
		Status:  "ACTIVE",
	})
	m.saveToFile()
	return nil
}

func (m *MockAWSClient) UpdateDNSRecord(ctx context.Context, zoneID, recordName, targetEndpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := fmt.Sprintf("UpdateDNSRecord:%s:%s->%s", zoneID, recordName, targetEndpoint)
	m.CreatedCalls = append(m.CreatedCalls, call)

	if m.DNSRecords[zoneID] == nil {
		m.DNSRecords[zoneID] = make(map[string]string)
	}
	cleanName := strings.TrimSuffix(recordName, ".")
	m.DNSRecords[zoneID][cleanName] = targetEndpoint
	m.saveToFile()
	return nil
}

func (m *MockAWSClient) GetDNSRecord(ctx context.Context, zoneID, recordName string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cleanName := strings.TrimSuffix(recordName, ".")
	if zone, ok := m.DNSRecords[zoneID]; ok {
		if val, exists := zone[cleanName]; exists {
			return val, nil
		}
	}
	return "", fmt.Errorf("dns record %q not found in zone %q", recordName, zoneID)
}

func (m *MockAWSClient) GetCreatedCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, len(m.CreatedCalls))
	copy(res, m.CreatedCalls)
	return res
}
