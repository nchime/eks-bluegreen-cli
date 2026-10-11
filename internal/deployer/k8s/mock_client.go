package k8s

import (
	"context"
	"sync"

	"eks-bluegreen-cli/pkg/models"
)

type MockK8sClient struct {
	mu             sync.RWMutex
	Workloads      *models.K8sWorkloadInfo
	AppliedItems   []string
	ReadyNodes     int
	TotalNodes     int
	KubeletVersion string
	ReadyPods      int
	TotalPods      int
	EndpointsReady bool
}

func NewMockK8sClient() *MockK8sClient {
	return &MockK8sClient{
		Workloads: &models.K8sWorkloadInfo{
			Namespaces: []string{"default", "ingress-nginx", "backend", "kube-system"},
			Deployments: []models.WorkloadItem{
				{
					Namespace:  "backend",
					Name:       "api-server",
					Replicas:   3,
					Images:     []string{"myorg/api:v1.2.0"},
					APIVersion: "apps/v1",
					Labels:     map[string]string{"app": "api-server"},
				},
				{
					Namespace:  "backend",
					Name:       "worker",
					Replicas:   2,
					Images:     []string{"myorg/worker:v1.0.4"},
					APIVersion: "apps/v1",
					Labels:     map[string]string{"app": "worker"},
				},
			},
			StatefulSets: []models.WorkloadItem{
				{
					Namespace:  "backend",
					Name:       "redis-cluster",
					Replicas:   3,
					Images:     []string{"redis:7.0-alpine"},
					APIVersion: "apps/v1",
					Labels:     map[string]string{"app": "redis"},
				},
			},
			DaemonSets: []models.WorkloadItem{
				{
					Namespace:  "kube-system",
					Name:       "aws-node",
					Replicas:   3,
					Images:     []string{"amazon-k8s-cni:v1.15.1"},
					APIVersion: "apps/v1",
				},
			},
			Services: []models.ServiceItem{
				{
					Namespace:   "backend",
					Name:        "api-service",
					Type:        "LoadBalancer",
					ClusterIP:   "172.20.100.5",
					ExternalDNS: "k8s-backend-apiserv-1234.ap-northeast-2.elb.amazonaws.com",
					Ports:       []int32{80, 443},
				},
			},
			Ingresses: []models.IngressItem{
				{
					Namespace: "backend",
					Name:      "api-ingress",
					Class:     "alb",
					Hosts:     []string{"app.example.com"},
					Address:   "k8s-alb-ingress-5678.ap-northeast-2.elb.amazonaws.com",
				},
			},
			StorageClasses: []string{"gp2", "gp3"},
			PVCs: []models.PVCItem{
				{
					Namespace:        "backend",
					Name:             "redis-data-0",
					StorageClass:     "gp3",
					RequestedStorage: "50Gi",
					Status:           "Bound",
				},
			},
			HelmReleases: []models.HelmItem{
				{
					Namespace:   "ingress-nginx",
					ReleaseName: "ingress-nginx",
					Chart:       "ingress-nginx-4.8.3",
					Version:     "4.8.3",
					Status:      "deployed",
				},
			},
		},
		AppliedItems:   []string{},
		ReadyNodes:     3,
		TotalNodes:     3,
		KubeletVersion: "v1.30.0",
		ReadyPods:      8,
		TotalPods:      8,
		EndpointsReady: true,
	}
}

func (m *MockK8sClient) InspectWorkloads(ctx context.Context) (*models.K8sWorkloadInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Workloads, nil
}

func (m *MockK8sClient) ApplyNamespace(ctx context.Context, namespace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AppliedItems = append(m.AppliedItems, "Namespace:"+namespace)
	return nil
}

func (m *MockK8sClient) ApplyWorkload(ctx context.Context, item models.WorkloadItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AppliedItems = append(m.AppliedItems, "Workload:"+item.Namespace+"/"+item.Name)
	return nil
}

func (m *MockK8sClient) ApplyService(ctx context.Context, item models.ServiceItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AppliedItems = append(m.AppliedItems, "Service:"+item.Namespace+"/"+item.Name)
	return nil
}

func (m *MockK8sClient) ApplyIngress(ctx context.Context, item models.IngressItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AppliedItems = append(m.AppliedItems, "Ingress:"+item.Namespace+"/"+item.Name)
	return nil
}

func (m *MockK8sClient) GetNodeStatus(ctx context.Context) (int, int, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ReadyNodes, m.TotalNodes, m.KubeletVersion, nil
}

func (m *MockK8sClient) GetPodReadiness(ctx context.Context, namespace string) (int, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ReadyPods, m.TotalPods, nil
}

func (m *MockK8sClient) CheckServiceEndpoints(ctx context.Context, namespace, serviceName string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.EndpointsReady, nil
}
