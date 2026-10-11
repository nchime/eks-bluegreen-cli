package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"eks-bluegreen-cli/pkg/models"
)

// GenerateInspectionMarkdown formats inspection results into a Markdown document.
func GenerateInspectionMarkdown(ins *models.ClusterInspection) string {
	var sb strings.Builder

	sb.WriteString("# EKS Cluster Inspection Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Cluster Name:** `%s`\n", ins.Cluster.Name))
	sb.WriteString(fmt.Sprintf("- **Kubernetes Version:** `%s`\n", ins.Cluster.Version))
	sb.WriteString(fmt.Sprintf("- **Status:** `%s`\n", ins.Cluster.Status))
	sb.WriteString(fmt.Sprintf("- **VPC ID:** `%s`\n", ins.Cluster.VpcID))
	sb.WriteString(fmt.Sprintf("- **Endpoint:** `%s`\n", ins.Cluster.Endpoint))
	sb.WriteString(fmt.Sprintf("- **Inspected At:** %s\n\n", ins.InspectedAt.Format(time.RFC3339)))

	sb.WriteString("## Managed Node Groups\n\n")
	if len(ins.NodeGroups) == 0 {
		sb.WriteString("No managed node groups found.\n\n")
	} else {
		sb.WriteString("| Name | Status | Instance Types | Desired/Min/Max | AMI Type |\n")
		sb.WriteString("|---|---|---|---|---|\n")
		for _, ng := range ins.NodeGroups {
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %d/%d/%d | %s |\n",
				ng.Name, ng.Status, strings.Join(ng.InstanceTypes, ","),
				ng.DesiredSize, ng.MinSize, ng.MaxSize, ng.AmiType))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## EKS Addons\n\n")
	if len(ins.Addons) == 0 {
		sb.WriteString("No addons installed.\n\n")
	} else {
		sb.WriteString("| Addon Name | Version | Status |\n")
		sb.WriteString("|---|---|---|\n")
		for _, a := range ins.Addons {
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", a.Name, a.Version, a.Status))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Workload Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Namespaces:** %d (%s)\n", len(ins.Workloads.Namespaces), strings.Join(ins.Workloads.Namespaces, ", ")))
	sb.WriteString(fmt.Sprintf("- **Deployments:** %d\n", len(ins.Workloads.Deployments)))
	sb.WriteString(fmt.Sprintf("- **StatefulSets:** %d\n", len(ins.Workloads.StatefulSets)))
	sb.WriteString(fmt.Sprintf("- **DaemonSets:** %d\n", len(ins.Workloads.DaemonSets)))
	sb.WriteString(fmt.Sprintf("- **Services:** %d\n", len(ins.Workloads.Services)))
	sb.WriteString(fmt.Sprintf("- **Ingresses:** %d\n", len(ins.Workloads.Ingresses)))
	sb.WriteString(fmt.Sprintf("- **PVCs:** %d\n", len(ins.Workloads.PVCs)))
	sb.WriteString(fmt.Sprintf("- **Helm Releases:** %d\n\n", len(ins.Workloads.HelmReleases)))

	if len(ins.DeprecatedAPIs) > 0 {
		sb.WriteString("## Deprecated API Warnings\n\n")
		sb.WriteString("| Resource Kind | API Version | Removed In | Replacement | Objects |\n")
		sb.WriteString("|---|---|---|---|---|\n")
		for _, w := range ins.DeprecatedAPIs {
			sb.WriteString(fmt.Sprintf("| %s | `%s` | %s | `%s` | %s |\n",
				w.ResourceKind, w.APIVersion, w.RemovedInVersion, w.ReplacementAPI, strings.Join(w.AffectedObjects, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GeneratePlanMarkdown formats an upgrade plan into Markdown.
func GeneratePlanMarkdown(plan *models.PlanReport) string {
	var sb strings.Builder

	sb.WriteString("# EKS Blue-Green Upgrade Plan\n\n")
	sb.WriteString(fmt.Sprintf("- **Job ID:** `%s`\n", plan.JobID))
	sb.WriteString(fmt.Sprintf("- **Source (Blue):** `%s` (v%s)\n", plan.SourceCluster, plan.SourceVersion))
	sb.WriteString(fmt.Sprintf("- **Target (Green):** `%s` (v%s)\n", plan.TargetCluster, plan.TargetVersion))
	sb.WriteString(fmt.Sprintf("- **Region:** `%s`\n", plan.Region))
	sb.WriteString(fmt.Sprintf("- **Plan Hash:** `%s`\n", plan.PlanHash))
	sb.WriteString(fmt.Sprintf("- **Upgrade Compatible:** %v\n\n", plan.Compatible))

	if len(plan.BlockingReasons) > 0 {
		sb.WriteString("### Blocking Reasons & Prerequisites\n\n")
		for _, b := range plan.BlockingReasons {
			sb.WriteString(fmt.Sprintf("- ⚠️ %s\n", b))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Resource Execution Plans\n\n")
	sb.WriteString("| Resource Type | Resource Name | Action | Risk Level | Notes |\n")
	sb.WriteString("|---|---|---|---|---|\n")
	for _, rp := range plan.ResourcePlans {
		sb.WriteString(fmt.Sprintf("| %s | `%s` | **%s** | %s | %s |\n",
			rp.ResourceType, rp.ResourceName, rp.Action, rp.RiskLevel, rp.Notes))
	}
	sb.WriteString("\n")

	sb.WriteString("## Estimated Green Infrastructure Cost\n\n")
	sb.WriteString(fmt.Sprintf("- Control Plane: $%.2f / month\n", plan.EstimatedCost.ControlPlaneMonthlyUSD))
	sb.WriteString(fmt.Sprintf("- Worker Nodes: $%.2f / month\n", plan.EstimatedCost.WorkerNodesMonthlyUSD))
	sb.WriteString(fmt.Sprintf("- **Estimated Total:** **$%.2f / month**\n\n", plan.EstimatedCost.EstimatedTotalMonthlyUSD))

	return sb.String()
}

// GenerateValidationMarkdown formats validation results.
func GenerateValidationMarkdown(jobID string, results []models.ValidationResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# EKS Blue-Green Validation Report (Job `%s`)\n\n", jobID))
	sb.WriteString("| Check Name | Severity | Status | Details |\n")
	sb.WriteString("|---|---|---|---|\n")
	for _, r := range results {
		icon := "✅"
		if r.Status == models.ValidationWarn {
			icon = "⚠️"
		} else if r.Status == models.ValidationFail {
			icon = "❌"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s %s | %s |\n",
			r.CheckName, r.Severity, icon, r.Status, r.Details))
	}
	sb.WriteString("\n")
	return sb.String()
}

// GenerateFullReportMarkdown outputs a comprehensive job audit report.
func GenerateFullReportMarkdown(job *models.UpgradeJob, steps []models.ExecutionStep, val []models.ValidationResult, app *models.ApprovalRecord) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# EKS Blue-Green Job Report (`%s`)\n\n", job.ID))
	sb.WriteString(fmt.Sprintf("- **Source (Blue):** `%s` (v%s)\n", job.SourceCluster, job.SourceVersion))
	sb.WriteString(fmt.Sprintf("- **Target (Green):** `%s` (v%s)\n", job.TargetCluster, job.TargetVersion))
	sb.WriteString(fmt.Sprintf("- **Current Phase:** **%s**\n", job.CurrentPhase))
	sb.WriteString(fmt.Sprintf("- **Status:** **%s**\n", job.Status))
	if job.LastError != "" {
		sb.WriteString(fmt.Sprintf("- **Last Error:** `%s`\n", job.LastError))
	}
	sb.WriteString(fmt.Sprintf("- **Created At:** %s\n", job.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Updated At:** %s\n\n", job.UpdatedAt.Format(time.RFC3339)))

	if app != nil {
		sb.WriteString("## Approval Record\n\n")
		sb.WriteString(fmt.Sprintf("- **Approver:** `%s`\n", app.Approver))
		sb.WriteString(fmt.Sprintf("- **Approved At:** %s\n", app.ApprovedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("- **Status:** %s\n", app.Status))
		sb.WriteString(fmt.Sprintf("- **Plan Hash:** `%s`\n", app.PlanHash))
		sb.WriteString(fmt.Sprintf("- **Validation Hash:** `%s`\n\n", app.ValidationHash))
	}

	sb.WriteString("## Execution Steps Audit Log\n\n")
	sb.WriteString("| Step Name | Status | Started | Completed | Output Reference | Error |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")
	for _, s := range steps {
		comp := "-"
		if s.CompletedAt != nil {
			comp = s.CompletedAt.Format("15:04:05")
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s |\n",
			s.StepName, s.Status, s.StartedAt.Format("15:04:05"), comp, s.OutputReference, s.ErrorMessage))
	}
	sb.WriteString("\n")

	if len(val) > 0 {
		sb.WriteString("## Validation Results\n\n")
		sb.WriteString(GenerateValidationMarkdown(job.ID, val))
	}

	return sb.String()
}

// ToJSON serializes any struct to indented JSON.
func ToJSON(v interface{}) (string, error) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
