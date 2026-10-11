package k8s

import (
	"context"

	"eks-bluegreen-cli/pkg/models"
)

// K8sClient abstracts Kubernetes API interactions.
type K8sClient interface {
	InspectWorkloads(ctx context.Context) (*models.K8sWorkloadInfo, error)
	ApplyNamespace(ctx context.Context, namespace string) error
	ApplyWorkload(ctx context.Context, item models.WorkloadItem) error
	ApplyService(ctx context.Context, item models.ServiceItem) error
	ApplyIngress(ctx context.Context, item models.IngressItem) error
	GetNodeStatus(ctx context.Context) (readyNodes int, totalNodes int, kubeletVersion string, err error)
	GetPodReadiness(ctx context.Context, namespace string) (readyPods int, totalPods int, err error)
	CheckServiceEndpoints(ctx context.Context, namespace, serviceName string) (bool, error)
}
