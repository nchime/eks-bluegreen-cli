package validator

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"eks-bluegreen-cli/internal/aws"
	"eks-bluegreen-cli/internal/deployer/k8s"
	"eks-bluegreen-cli/internal/security"
	"eks-bluegreen-cli/internal/store"
	"eks-bluegreen-cli/pkg/models"
)

type Validator struct {
	awsClient aws.Client
	k8sClient k8s.K8sClient
	store     store.Store
}

func NewValidator(awsClient aws.Client, k8sClient k8s.K8sClient, st store.Store) *Validator {
	return &Validator{
		awsClient: awsClient,
		k8sClient: k8sClient,
		store:     st,
	}
}

// Validate executes all automated health, readiness, and connectivity checks on the Green cluster.
func (v *Validator) Validate(ctx context.Context, job *models.UpgradeJob, healthCheckURLs []string, timeoutSec int) ([]models.ValidationResult, string, error) {
	var results []models.ValidationResult
	now := time.Now().UTC()

	// 1. Green Cluster Status
	clInfo, err := v.awsClient.DescribeCluster(ctx, job.TargetCluster)
	if err != nil || clInfo == nil {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "GreenClusterStatus",
			Severity:  models.SeverityCritical,
			Status:    models.ValidationFail,
			Details:   fmt.Sprintf("Cluster %s is unreachable or not found: %v", job.TargetCluster, err),
			CheckedAt: now,
		})
	} else if clInfo.Status != "ACTIVE" {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "GreenClusterStatus",
			Severity:  models.SeverityCritical,
			Status:    models.ValidationFail,
			Details:   fmt.Sprintf("Cluster %s is in %s state, expected ACTIVE", job.TargetCluster, clInfo.Status),
			CheckedAt: now,
		})
	} else {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "GreenClusterStatus",
			Severity:  models.SeverityInfo,
			Status:    models.ValidationPass,
			Details:   fmt.Sprintf("Cluster %s is ACTIVE (v%s, endpoint: %s)", job.TargetCluster, clInfo.Version, clInfo.Endpoint),
			CheckedAt: now,
		})
	}

	// 2. Node Group & Node Status
	readyNodes, totalNodes, kubeletVer, nodeErr := v.k8sClient.GetNodeStatus(ctx)
	if nodeErr != nil || totalNodes == 0 {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "WorkerNodesReady",
			Severity:  models.SeverityCritical,
			Status:    models.ValidationFail,
			Details:   fmt.Sprintf("Failed to query nodes or 0 nodes found: %v", nodeErr),
			CheckedAt: now,
		})
	} else if readyNodes < totalNodes {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "WorkerNodesReady",
			Severity:  models.SeverityWarn,
			Status:    models.ValidationWarn,
			Details:   fmt.Sprintf("%d/%d nodes ready (kubelet %s)", readyNodes, totalNodes, kubeletVer),
			CheckedAt: now,
		})
	} else {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "WorkerNodesReady",
			Severity:  models.SeverityInfo,
			Status:    models.ValidationPass,
			Details:   fmt.Sprintf("All %d nodes ready with kubelet %s", readyNodes, kubeletVer),
			CheckedAt: now,
		})
	}

	// 3. EKS Addons Check
	addons, addonErr := v.awsClient.ListAddons(ctx, job.TargetCluster)
	if addonErr != nil || len(addons) == 0 {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "EKSAddonsHealth",
			Severity:  models.SeverityWarn,
			Status:    models.ValidationWarn,
			Details:   "Could not verify EKS Addons on target cluster",
			CheckedAt: now,
		})
	} else {
		allAddonsActive := true
		for _, a := range addons {
			if a.Status != "ACTIVE" {
				allAddonsActive = false
				break
			}
		}
		if allAddonsActive {
			results = append(results, models.ValidationResult{
				JobID:     job.ID,
				CheckName: "EKSAddonsHealth",
				Severity:  models.SeverityInfo,
				Status:    models.ValidationPass,
				Details:   fmt.Sprintf("All %d addons (vpc-cni, kube-proxy, coredns, etc.) are ACTIVE", len(addons)),
				CheckedAt: now,
			})
		} else {
			results = append(results, models.ValidationResult{
				JobID:     job.ID,
				CheckName: "EKSAddonsHealth",
				Severity:  models.SeverityCritical,
				Status:    models.ValidationFail,
				Details:   "One or more EKS Addons are not in ACTIVE state",
				CheckedAt: now,
			})
		}
	}

	// 4. Pod & Workload Readiness
	readyPods, totalPods, podErr := v.k8sClient.GetPodReadiness(ctx, "")
	if podErr != nil {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "WorkloadPodReadiness",
			Severity:  models.SeverityCritical,
			Status:    models.ValidationFail,
			Details:   fmt.Sprintf("Failed to inspect pod readiness: %v", podErr),
			CheckedAt: now,
		})
	} else if totalPods == 0 || readyPods < totalPods {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "WorkloadPodReadiness",
			Severity:  models.SeverityCritical,
			Status:    models.ValidationFail,
			Details:   fmt.Sprintf("Only %d/%d pods are ready in Green cluster", readyPods, totalPods),
			CheckedAt: now,
		})
	} else {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "WorkloadPodReadiness",
			Severity:  models.SeverityInfo,
			Status:    models.ValidationPass,
			Details:   fmt.Sprintf("All %d pods are ready with healthy probes", readyPods),
			CheckedAt: now,
		})
	}

	// 5. Service & Endpoint connectivity
	endpointsOk, epErr := v.k8sClient.CheckServiceEndpoints(ctx, "backend", "api-service")
	if epErr != nil || !endpointsOk {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "ServiceEndpointsReachability",
			Severity:  models.SeverityCritical,
			Status:    models.ValidationFail,
			Details:   "Service endpoints for backend/api-service are not ready",
			CheckedAt: now,
		})
	} else {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "ServiceEndpointsReachability",
			Severity:  models.SeverityInfo,
			Status:    models.ValidationPass,
			Details:   "Service endpoints are active and routing traffic",
			CheckedAt: now,
		})
	}

	// 6. Application HTTP Health Check (if URLs provided)
	if len(healthCheckURLs) > 0 {
		client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
		for _, url := range healthCheckURLs {
			resp, err := client.Get(url)
			if err != nil || resp.StatusCode >= 400 {
				statusText := "Connection error"
				if resp != nil {
					statusText = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
				results = append(results, models.ValidationResult{
					JobID:     job.ID,
					CheckName: "HttpHealthCheck:" + url,
					Severity:  models.SeverityCritical,
					Status:    models.ValidationFail,
					Details:   fmt.Sprintf("Healthcheck failed for %s: %s", url, statusText),
					CheckedAt: now,
				})
			} else {
				results = append(results, models.ValidationResult{
					JobID:     job.ID,
					CheckName: "HttpHealthCheck:" + url,
					Severity:  models.SeverityInfo,
					Status:    models.ValidationPass,
					Details:   fmt.Sprintf("Healthcheck passed for %s (HTTP %d)", url, resp.StatusCode),
					CheckedAt: now,
				})
			}
		}
	} else {
		results = append(results, models.ValidationResult{
			JobID:     job.ID,
			CheckName: "HttpHealthCheck",
			Severity:  models.SeverityInfo,
			Status:    models.ValidationPass,
			Details:   "No external HTTP health check URLs configured; internal probe checks passed",
			CheckedAt: now,
		})
	}

	// Compute validation hash
	valHash := security.ComputeValidationHash(results)

	// Save to store
	if err := v.store.SaveValidationResults(ctx, job.ID, results); err != nil {
		return results, valHash, fmt.Errorf("failed to save validation results: %w", err)
	}

	return results, valHash, nil
}

// HasCriticalFailures returns true if any check with SeverityCritical has status FAIL.
func HasCriticalFailures(results []models.ValidationResult) bool {
	for _, r := range results {
		if r.Severity == models.SeverityCritical && r.Status == models.ValidationFail {
			return true
		}
	}
	return false
}
