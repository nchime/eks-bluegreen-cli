package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"
)

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9_\-\.]{15,}`),
	regexp.MustCompile(`(?i)(aws_access_key_id\s*[:=]\s*)(AKIA[0-9A-Z]{16})`),
	regexp.MustCompile(`(?i)(aws_secret_access_key\s*[:=]\s*)[a-zA-Z0-9/+=]{40}`),
	regexp.MustCompile(`(?i)(aws_session_token\s*[:=]\s*)[a-zA-Z0-9/+=]{50,}`),
	regexp.MustCompile(`(?i)("password"|"token"|"secret"|"client_secret"|"secret_key")\s*:\s*"[^"]+"`),
	regexp.MustCompile(`(?i)\b(password|token|client_secret|secret_key|api[_-]?key)\s*[:=]\s*([^\s;]+)`),
	regexp.MustCompile(`(?i)\b(secret)\s*[:=]\s*([^:\s;]+)`),
}

// MaskSensitiveData scans text and masks tokens, passwords, and AWS credentials.
func MaskSensitiveData(input string) string {
	masked := input
	for _, pattern := range sensitivePatterns {
		masked = pattern.ReplaceAllStringFunc(masked, func(m string) string {
			parts := strings.SplitN(m, ":", 2)
			if len(parts) == 2 {
				return parts[0] + ": [REDACTED]"
			}
			partsEq := strings.SplitN(m, "=", 2)
			if len(partsEq) == 2 {
				return partsEq[0] + "= [REDACTED]"
			}
			if strings.HasPrefix(strings.ToLower(m), "bearer ") {
				return "Bearer [REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return masked
}

// AssertSafeOperation verifies that the target cluster is not the Blue cluster.
func AssertSafeOperation(targetCluster, blueCluster, operation string) error {
	if strings.TrimSpace(targetCluster) == "" {
		return &apperrors.GuardrailError{
			ClusterName: targetCluster,
			Operation:   operation,
			Reason:      "target cluster name cannot be empty",
		}
	}
	if strings.EqualFold(strings.TrimSpace(targetCluster), strings.TrimSpace(blueCluster)) {
		return &apperrors.GuardrailError{
			ClusterName: targetCluster,
			Operation:   operation,
			Reason:      "target cluster cannot be identical to source (blue) cluster; destruction or in-place mutation of blue cluster is strictly prohibited",
		}
	}
	return nil
}

// ValidateBlueClusterProtection checks if any destructive action is attempted on Blue cluster.
func ValidateBlueClusterProtection(clusterName, blueCluster string) error {
	if strings.EqualFold(strings.TrimSpace(clusterName), strings.TrimSpace(blueCluster)) {
		return apperrors.ErrBlueClusterDeletion
	}
	return nil
}

// ComputePlanHash generates a deterministic SHA-256 hash of the plan.
func ComputePlanHash(plan *models.PlanReport) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("plan is nil")
	}

	// Create canonical copy for stable hashing
	type canonicalPlan struct {
		SourceCluster string                `json:"source_cluster"`
		SourceVersion string                `json:"source_version"`
		TargetCluster string                `json:"target_cluster"`
		TargetVersion string                `json:"target_version"`
		Region        string                `json:"region"`
		ResourcePlans []models.ResourcePlan `json:"resource_plans"`
	}

	plansCopy := append([]models.ResourcePlan(nil), plan.ResourcePlans...)
	sort.Slice(plansCopy, func(i, j int) bool {
		if plansCopy[i].ResourceType == plansCopy[j].ResourceType {
			return plansCopy[i].ResourceName < plansCopy[j].ResourceName
		}
		return plansCopy[i].ResourceType < plansCopy[j].ResourceType
	})

	cp := canonicalPlan{
		SourceCluster: plan.SourceCluster,
		SourceVersion: plan.SourceVersion,
		TargetCluster: plan.TargetCluster,
		TargetVersion: plan.TargetVersion,
		Region:        plan.Region,
		ResourcePlans: plansCopy,
	}

	bytes, err := json.Marshal(cp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal plan for hashing: %w", err)
	}

	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:]), nil
}

// ComputeValidationHash generates a deterministic SHA-256 hash of the validation results.
func ComputeValidationHash(results []models.ValidationResult) string {
	resCopy := append([]models.ValidationResult(nil), results...)
	sort.Slice(resCopy, func(i, j int) bool {
		return resCopy[i].CheckName < resCopy[j].CheckName
	})

	type canonicalCheck struct {
		CheckName string                  `json:"check_name"`
		Status    models.ValidationStatus `json:"status"`
		Severity  models.Severity         `json:"severity"`
	}

	canonical := make([]canonicalCheck, len(resCopy))
	for i, r := range resCopy {
		canonical[i] = canonicalCheck{
			CheckName: r.CheckName,
			Status:    r.Status,
			Severity:  r.Severity,
		}
	}

	bytes, _ := json.Marshal(canonical)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// SafeExec runs an external command with arguments slice (no shell injection) and masks output.
func SafeExec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	cleanedOutput := MaskSensitiveData(string(output))
	if err != nil {
		return cleanedOutput, fmt.Errorf("command execution failed (%s): %w\nOutput: %s", name, err, cleanedOutput)
	}
	return cleanedOutput, nil
}
