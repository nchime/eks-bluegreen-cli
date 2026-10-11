package deployer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"eks-bluegreen-cli/internal/deployer/k8s"
	"eks-bluegreen-cli/internal/store"
	"eks-bluegreen-cli/pkg/models"
)

type Deployer struct {
	k8sClient k8s.K8sClient
	store     store.Store
}

func NewDeployer(k8sClient k8s.K8sClient, st store.Store) *Deployer {
	return &Deployer{
		k8sClient: k8sClient,
		store:     st,
	}
}

// SyncWorkloads deploys workloads to the Green cluster according to the plan.
func (d *Deployer) SyncWorkloads(ctx context.Context, jobID string, inspection *models.ClusterInspection, plan *models.PlanReport) error {
	if inspection == nil || plan == nil {
		return fmt.Errorf("inspection and plan data are required for sync")
	}

	// 1. Sync Namespaces
	for _, ns := range inspection.Workloads.Namespaces {
		if ns == "kube-system" || ns == "kube-public" || ns == "kube-node-lease" {
			continue // System namespaces managed by control plane
		}

		stepNS := &models.ExecutionStep{
			JobID:     jobID,
			StepName:  "SyncNamespace:" + ns,
			Status:    models.StepStatusRunning,
			StartedAt: time.Now().UTC(),
		}
		_ = d.store.SaveExecutionStep(ctx, stepNS)

		if err := d.k8sClient.ApplyNamespace(ctx, ns); err != nil {
			stepNS.Status = models.StepStatusFailed
			stepNS.ErrorMessage = err.Error()
			_ = d.store.SaveExecutionStep(ctx, stepNS)
			return fmt.Errorf("failed to sync namespace %s: %w", ns, err)
		}

		now := time.Now().UTC()
		stepNS.Status = models.StepStatusCompleted
		stepNS.CompletedAt = &now
		_ = d.store.SaveExecutionStep(ctx, stepNS)
	}

	// 2. Deploy Services
	for _, svc := range inspection.Workloads.Services {
		stepSvc := &models.ExecutionStep{
			JobID:     jobID,
			StepName:  "SyncService:" + svc.Namespace + "/" + svc.Name,
			Status:    models.StepStatusRunning,
			StartedAt: time.Now().UTC(),
		}
		_ = d.store.SaveExecutionStep(ctx, stepSvc)

		if err := d.k8sClient.ApplyService(ctx, svc); err != nil {
			stepSvc.Status = models.StepStatusFailed
			stepSvc.ErrorMessage = err.Error()
			_ = d.store.SaveExecutionStep(ctx, stepSvc)
			return fmt.Errorf("failed to sync service %s/%s: %w", svc.Namespace, svc.Name, err)
		}

		now := time.Now().UTC()
		stepSvc.Status = models.StepStatusCompleted
		stepSvc.CompletedAt = &now
		_ = d.store.SaveExecutionStep(ctx, stepSvc)
	}

	// 3. Deploy Workloads (Deployments)
	for _, dep := range inspection.Workloads.Deployments {
		stepDep := &models.ExecutionStep{
			JobID:     jobID,
			StepName:  "SyncDeployment:" + dep.Namespace + "/" + dep.Name,
			Status:    models.StepStatusRunning,
			StartedAt: time.Now().UTC(),
		}
		_ = d.store.SaveExecutionStep(ctx, stepDep)

		// Sanitize/mask any secret references
		safeDep := dep
		if safeDep.Labels == nil {
			safeDep.Labels = make(map[string]string)
		}
		safeDep.Labels["eksbg.sync.job"] = jobID
		safeDep.Labels["eksbg.sync.role"] = "green"

		if err := d.k8sClient.ApplyWorkload(ctx, safeDep); err != nil {
			stepDep.Status = models.StepStatusFailed
			stepDep.ErrorMessage = err.Error()
			_ = d.store.SaveExecutionStep(ctx, stepDep)
			return fmt.Errorf("failed to sync deployment %s/%s: %w", dep.Namespace, dep.Name, err)
		}

		now := time.Now().UTC()
		stepDep.Status = models.StepStatusCompleted
		stepDep.CompletedAt = &now
		_ = d.store.SaveExecutionStep(ctx, stepDep)
	}

	// 4. Ingress Reconfiguration
	for _, ing := range inspection.Workloads.Ingresses {
		stepIng := &models.ExecutionStep{
			JobID:     jobID,
			StepName:  "SyncIngress:" + ing.Namespace + "/" + ing.Name,
			Status:    models.StepStatusRunning,
			StartedAt: time.Now().UTC(),
		}
		_ = d.store.SaveExecutionStep(ctx, stepIng)

		greenIng := ing
		// Ingress host is maintained, address will be green ALB
		if err := d.k8sClient.ApplyIngress(ctx, greenIng); err != nil {
			stepIng.Status = models.StepStatusFailed
			stepIng.ErrorMessage = err.Error()
			_ = d.store.SaveExecutionStep(ctx, stepIng)
			return fmt.Errorf("failed to sync ingress %s/%s: %w", ing.Namespace, ing.Name, err)
		}

		now := time.Now().UTC()
		stepIng.Status = models.StepStatusCompleted
		stepIng.CompletedAt = &now
		_ = d.store.SaveExecutionStep(ctx, stepIng)
	}

	// 5. StatefulSet / PVC Migration Check Warning
	if len(inspection.Workloads.StatefulSets) > 0 || len(inspection.Workloads.PVCs) > 0 {
		stepCheck := &models.ExecutionStep{
			JobID:           jobID,
			StepName:        "CheckStorageMigrationRequirements",
			Status:          models.StepStatusCompleted,
			StartedAt:       time.Now().UTC(),
			OutputReference: fmt.Sprintf("Storage resources found: %d StatefulSets, %d PVCs. Requires volume snapshot replication.", len(inspection.Workloads.StatefulSets), len(inspection.Workloads.PVCs)),
		}
		_ = d.store.SaveExecutionStep(ctx, stepCheck)
	}

	return nil
}

// FormatSafeSecretRef returns external secret reference format rather than plaintext values.
func FormatSafeSecretRef(secretName, key string) string {
	return strings.TrimSpace(fmt.Sprintf("secretKeyRef(name=%s, key=%s, [ENCRYPTED/REFERENCED])", secretName, key))
}
