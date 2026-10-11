package models

import "time"

// JobPhase represents the lifecycle phases of an upgrade job.
type JobPhase string

const (
	PhaseInit                JobPhase = "INIT"
	PhaseInspected           JobPhase = "INSPECTED"
	PhasePlanned             JobPhase = "PLANNED"
	PhaseProvisioning        JobPhase = "PROVISIONING"
	PhaseProvisioned         JobPhase = "PROVISIONED"
	PhaseSyncing             JobPhase = "SYNCING"
	PhaseSynced              JobPhase = "SYNCED"
	PhaseValidating          JobPhase = "VALIDATING"
	PhaseValidated           JobPhase = "VALIDATED"
	PhaseWaitingForApproval  JobPhase = "WAITING_FOR_APPROVAL"
	PhaseApproved            JobPhase = "APPROVED"
	PhaseCutoverInProgress   JobPhase = "CUTOVER_IN_PROGRESS"
	PhaseCutoverCompleted    JobPhase = "CUTOVER_COMPLETED"
	PhaseRecovered           JobPhase = "RECOVERED"
)

// JobStatus represents the health/execution status of the current phase.
type JobStatus string

const (
	StatusPending     JobStatus = "PENDING"
	StatusRunning     JobStatus = "RUNNING"
	StatusCompleted   JobStatus = "COMPLETED"
	StatusFailed      JobStatus = "FAILED"
	StatusRecoverable JobStatus = "RECOVERABLE"
)

// UpgradeJob tracks the state of a blue-green upgrade operation.
type UpgradeJob struct {
	ID            string    `json:"id" yaml:"id"`
	SourceCluster string    `json:"source_cluster" yaml:"source_cluster"`
	SourceRegion  string    `json:"source_region" yaml:"source_region"`
	TargetCluster string    `json:"target_cluster" yaml:"target_cluster"`
	TargetRegion  string    `json:"target_region" yaml:"target_region"`
	SourceVersion string    `json:"source_version" yaml:"source_version"`
	TargetVersion string    `json:"target_version" yaml:"target_version"`
	CurrentPhase  JobPhase  `json:"current_phase" yaml:"current_phase"`
	Status        JobStatus `json:"status" yaml:"status"`
	PlanPath      string    `json:"plan_path,omitempty" yaml:"plan_path,omitempty"`
	PlanHash      string    `json:"plan_hash,omitempty" yaml:"plan_hash,omitempty"`
	CreatedAt     time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" yaml:"updated_at"`
	LastError     string    `json:"last_error,omitempty" yaml:"last_error,omitempty"`
}

// ActionType defines the action to take for a resource.
type ActionType string

const (
	ActionReplicate   ActionType = "REPLICATE"
	ActionReconfigure ActionType = "RECONFIGURE"
	ActionExclude     ActionType = "EXCLUDE"
	ActionManualCheck ActionType = "MANUAL_CHECK"
)

// RiskLevel defines the risk severity of replicating/migrating a resource.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// ResourcePlan represents an individual resource migration plan item.
type ResourcePlan struct {
	JobID            string     `json:"job_id" yaml:"job_id"`
	ResourceType     string     `json:"resource_type" yaml:"resource_type"`
	ResourceName     string     `json:"resource_name" yaml:"resource_name"`
	Action           ActionType `json:"action" yaml:"action"`
	Dependencies     []string   `json:"dependencies" yaml:"dependencies"`
	RiskLevel        RiskLevel  `json:"risk_level" yaml:"risk_level"`
	ValidationStatus string     `json:"validation_status" yaml:"validation_status"`
	Notes            string     `json:"notes" yaml:"notes"`
}

// ApprovalStatus represents the status of an approval record.
type ApprovalStatus string

const (
	ApprovalStatusApproved ApprovalStatus = "APPROVED"
	ApprovalStatusRejected ApprovalStatus = "REJECTED"
	ApprovalStatusRevoked  ApprovalStatus = "REVOKED"
)

// ApprovalRecord records user confirmation for traffic cutover.
type ApprovalRecord struct {
	JobID          string         `json:"job_id" yaml:"job_id"`
	ApprovalType   string         `json:"approval_type" yaml:"approval_type"`
	Approver       string         `json:"approver" yaml:"approver"`
	ApprovedAt     time.Time      `json:"approved_at" yaml:"approved_at"`
	PlanHash       string         `json:"plan_hash" yaml:"plan_hash"`
	ValidationHash string         `json:"validation_hash" yaml:"validation_hash"`
	Status         ApprovalStatus `json:"status" yaml:"status"`
	Comments       string         `json:"comments,omitempty" yaml:"comments,omitempty"`
}

// StepStatus represents the outcome of an execution step.
type StepStatus string

const (
	StepStatusPending   StepStatus = "PENDING"
	StepStatusRunning   StepStatus = "RUNNING"
	StepStatusCompleted StepStatus = "COMPLETED"
	StepStatusFailed    StepStatus = "FAILED"
	StepStatusSkipped   StepStatus = "SKIPPED"
)

// ExecutionStep records individual sub-tasks in a workflow.
type ExecutionStep struct {
	ID              int64      `json:"id" yaml:"id"`
	JobID           string     `json:"job_id" yaml:"job_id"`
	StepName        string     `json:"step_name" yaml:"step_name"`
	Status          StepStatus `json:"status" yaml:"status"`
	StartedAt       time.Time  `json:"started_at" yaml:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	RetryCount      int        `json:"retry_count" yaml:"retry_count"`
	ErrorMessage    string     `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	OutputReference string     `json:"output_reference,omitempty" yaml:"output_reference,omitempty"`
}

// Severity represents validation check impact level.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarn     Severity = "WARN"
	SeverityCritical Severity = "CRITICAL"
)

// ValidationStatus represents the check status.
type ValidationStatus string

const (
	ValidationPass ValidationStatus = "PASS"
	ValidationWarn ValidationStatus = "WARN"
	ValidationFail ValidationStatus = "FAIL"
)

// ValidationResult represents an individual health/readiness check.
type ValidationResult struct {
	JobID     string           `json:"job_id" yaml:"job_id"`
	CheckName string           `json:"check_name" yaml:"check_name"`
	Severity  Severity         `json:"severity" yaml:"severity"`
	Status    ValidationStatus `json:"status" yaml:"status"`
	Details   string           `json:"details" yaml:"details"`
	CheckedAt time.Time        `json:"checked_at" yaml:"checked_at"`
}

// EKSClusterInfo describes inspected EKS cluster metadata.
type EKSClusterInfo struct {
	Name            string            `json:"name" yaml:"name"`
	Arn             string            `json:"arn" yaml:"arn"`
	Version         string            `json:"version" yaml:"version"`
	Endpoint        string            `json:"endpoint" yaml:"endpoint"`
	RoleArn         string            `json:"role_arn" yaml:"role_arn"`
	Status          string            `json:"status" yaml:"status"`
	VpcID           string            `json:"vpc_id" yaml:"vpc_id"`
	SubnetIDs       []string          `json:"subnet_ids" yaml:"subnet_ids"`
	SecurityGroupIDs []string         `json:"security_group_ids" yaml:"security_group_ids"`
	OIDCProvider    string            `json:"oidc_provider" yaml:"oidc_provider"`
	Tags            map[string]string `json:"tags" yaml:"tags"`
}

// NodeGroupInfo describes an EKS Managed Node Group.
type NodeGroupInfo struct {
	Name          string            `json:"name" yaml:"name"`
	ClusterName   string            `json:"cluster_name" yaml:"cluster_name"`
	Status        string            `json:"status" yaml:"status"`
	CapacityType  string            `json:"capacity_type" yaml:"capacity_type"`
	InstanceTypes []string          `json:"instance_types" yaml:"instance_types"`
	DesiredSize   int32             `json:"desired_size" yaml:"desired_size"`
	MinSize       int32             `json:"min_size" yaml:"min_size"`
	MaxSize       int32             `json:"max_size" yaml:"max_size"`
	Subnets       []string          `json:"subnets" yaml:"subnets"`
	NodeRole      string            `json:"node_role" yaml:"node_role"`
	AmiType       string            `json:"ami_type" yaml:"ami_type"`
	ReleaseVersion string           `json:"release_version" yaml:"release_version"`
	Labels        map[string]string `json:"labels" yaml:"labels"`
}

// AddonInfo describes an EKS Add-on.
type AddonInfo struct {
	Name             string `json:"name" yaml:"name"`
	Version          string `json:"version" yaml:"version"`
	Status           string `json:"status" yaml:"status"`
	ServiceAccountArn string `json:"service_account_arn,omitempty" yaml:"service_account_arn,omitempty"`
}

// NetworkInfo describes VPC and subnet details.
type NetworkInfo struct {
	VpcID       string              `json:"vpc_id" yaml:"vpc_id"`
	CidrBlock   string              `json:"cidr_block" yaml:"cidr_block"`
	Subnets     []SubnetDetail      `json:"subnets" yaml:"subnets"`
	SecGroups   []SecurityGroupDetail `json:"security_groups" yaml:"security_groups"`
}

// SubnetDetail holds subnet capacity information.
type SubnetDetail struct {
	SubnetID         string `json:"subnet_id" yaml:"subnet_id"`
	AvailabilityZone string `json:"availability_zone" yaml:"availability_zone"`
	CidrBlock        string `json:"cidr_block" yaml:"cidr_block"`
	AvailableIPs     int32  `json:"available_ips" yaml:"available_ips"`
	IsPublic         bool   `json:"is_public" yaml:"is_public"`
}

// SecurityGroupDetail holds security group summary.
type SecurityGroupDetail struct {
	GroupID     string `json:"group_id" yaml:"group_id"`
	GroupName   string `json:"group_name" yaml:"group_name"`
	Description string `json:"description" yaml:"description"`
}

// K8sWorkloadInfo describes inspected Kubernetes resources.
type K8sWorkloadInfo struct {
	Namespaces    []string       `json:"namespaces" yaml:"namespaces"`
	Deployments   []WorkloadItem `json:"deployments" yaml:"deployments"`
	StatefulSets  []WorkloadItem `json:"stateful_sets" yaml:"stateful_sets"`
	DaemonSets    []WorkloadItem `json:"daemon_sets" yaml:"daemon_sets"`
	Services      []ServiceItem  `json:"services" yaml:"services"`
	Ingresses     []IngressItem  `json:"ingresses" yaml:"ingresses"`
	StorageClasses []string      `json:"storage_classes" yaml:"storage_classes"`
	PVCs          []PVCItem      `json:"pvcs" yaml:"pvcs"`
	HelmReleases  []HelmItem     `json:"helm_releases" yaml:"helm_releases"`
}

// WorkloadItem represents a k8s workload.
type WorkloadItem struct {
	Namespace string            `json:"namespace" yaml:"namespace"`
	Name      string            `json:"name" yaml:"name"`
	Replicas  int32             `json:"replicas" yaml:"replicas"`
	Images    []string          `json:"images" yaml:"images"`
	APIVersion string           `json:"api_version" yaml:"api_version"`
	Labels    map[string]string `json:"labels" yaml:"labels"`
}

// ServiceItem represents a k8s Service.
type ServiceItem struct {
	Namespace   string `json:"namespace" yaml:"namespace"`
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	ClusterIP   string `json:"cluster_ip" yaml:"cluster_ip"`
	ExternalDNS string `json:"external_dns,omitempty" yaml:"external_dns,omitempty"`
	Ports       []int32 `json:"ports" yaml:"ports"`
}

// IngressItem represents a k8s Ingress.
type IngressItem struct {
	Namespace string   `json:"namespace" yaml:"namespace"`
	Name      string   `json:"name" yaml:"name"`
	Class     string   `json:"class" yaml:"class"`
	Hosts     []string `json:"hosts" yaml:"hosts"`
	Address   string   `json:"address,omitempty" yaml:"address,omitempty"`
}

// PVCItem represents a PersistentVolumeClaim.
type PVCItem struct {
	Namespace        string `json:"namespace" yaml:"namespace"`
	Name             string `json:"name" yaml:"name"`
	StorageClass     string `json:"storage_class" yaml:"storage_class"`
	RequestedStorage string `json:"requested_storage" yaml:"requested_storage"`
	Status           string `json:"status" yaml:"status"`
}

// HelmItem represents a deployed Helm release.
type HelmItem struct {
	Namespace   string `json:"namespace" yaml:"namespace"`
	ReleaseName string `json:"release_name" yaml:"release_name"`
	Chart       string `json:"chart" yaml:"chart"`
	Version     string `json:"version" yaml:"version"`
	Status      string `json:"status" yaml:"status"`
}

// DeprecatedAPIWarning represents a detected deprecated or removed API.
type DeprecatedAPIWarning struct {
	ResourceKind    string `json:"resource_kind" yaml:"resource_kind"`
	APIVersion      string `json:"api_version" yaml:"api_version"`
	RemovedInVersion string `json:"removed_in_version" yaml:"removed_in_version"`
	ReplacementAPI  string `json:"replacement_api" yaml:"replacement_api"`
	AffectedObjects []string `json:"affected_objects" yaml:"affected_objects"`
}

// ClusterInspection aggregates all inspection results.
type ClusterInspection struct {
	Cluster        EKSClusterInfo         `json:"cluster" yaml:"cluster"`
	NodeGroups     []NodeGroupInfo        `json:"node_groups" yaml:"node_groups"`
	Addons         []AddonInfo            `json:"addons" yaml:"addons"`
	Network        NetworkInfo            `json:"network" yaml:"network"`
	Workloads      K8sWorkloadInfo        `json:"workloads" yaml:"workloads"`
	DeprecatedAPIs []DeprecatedAPIWarning `json:"deprecated_apis" yaml:"deprecated_apis"`
	InspectedAt    time.Time              `json:"inspected_at" yaml:"inspected_at"`
}

// CostEstimate holds estimated Green cluster run costs.
type CostEstimate struct {
	ControlPlaneMonthlyUSD float64 `json:"control_plane_monthly_usd" yaml:"control_plane_monthly_usd"`
	WorkerNodesMonthlyUSD  float64 `json:"worker_nodes_monthly_usd" yaml:"worker_nodes_monthly_usd"`
	EstimatedTotalMonthlyUSD float64 `json:"estimated_total_monthly_usd" yaml:"estimated_total_monthly_usd"`
}

// PlanReport summarizes the upgrade plan.
type PlanReport struct {
	JobID              string                 `json:"job_id" yaml:"job_id"`
	SourceCluster      string                 `json:"source_cluster" yaml:"source_cluster"`
	SourceVersion      string                 `json:"source_version" yaml:"source_version"`
	TargetCluster      string                 `json:"target_cluster" yaml:"target_cluster"`
	TargetVersion      string                 `json:"target_version" yaml:"target_version"`
	Region             string                 `json:"region" yaml:"region"`
	Compatible         bool                   `json:"compatible" yaml:"compatible"`
	BlockingReasons    []string               `json:"blocking_reasons,omitempty" yaml:"blocking_reasons,omitempty"`
	DeprecatedWarnings []DeprecatedAPIWarning `json:"deprecated_warnings,omitempty" yaml:"deprecated_warnings,omitempty"`
	AddonCompatibility []AddonCompatibilityCheck `json:"addon_compatibility" yaml:"addon_compatibility"`
	ResourcePlans      []ResourcePlan         `json:"resource_plans" yaml:"resource_plans"`
	EstimatedCost      CostEstimate           `json:"estimated_cost" yaml:"estimated_cost"`
	PlanHash           string                 `json:"plan_hash" yaml:"plan_hash"`
	CreatedAt          time.Time              `json:"created_at" yaml:"created_at"`
}

// AddonCompatibilityCheck stores addon version compatibility.
type AddonCompatibilityCheck struct {
	Name            string `json:"name" yaml:"name"`
	CurrentVersion  string `json:"current_version" yaml:"current_version"`
	TargetVersion   string `json:"target_version" yaml:"target_version"`
	IsCompatible    bool   `json:"is_compatible" yaml:"is_compatible"`
	Recommendation  string `json:"recommendation" yaml:"recommendation"`
}
