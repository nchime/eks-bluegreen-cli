# EKS Blue-Green Upgrade CLI (`eksbg`)

[English](README.md) | [한국어](README.ko.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org)
[![Tests](https://img.shields.io/badge/Tests-15%20passed-brightgreen.svg)](#test-results)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

`eksbg` is an open-source, production-grade CLI designed to eliminate downtime, control plane degradation, and operational risks during **Amazon EKS Kubernetes version upgrades**.

It analyzes your existing EKS cluster (Blue), creates a new cluster with the target Kubernetes version (Green), replicates workloads and configurations with safety guardrails, validates cluster readiness, and executes traffic cutover upon explicit human approval.

---

## 🚀 Key Features

- **Read-Only Blue Guardrail**: Blue clusters are strictly read-only; destructive operations or automated deletions are completely prohibited.
- **Automated Green Lifecycle**: Automated provisioning of EKS Control Plane, Managed Node Groups, and Add-ons with full resumption on failure.
- **API Deprecation Detection**: Automatically scans workloads for deprecated or removed Kubernetes APIs against target versions.
- **Safe Secret Handling**: Secrets are processed via safe external references and never exposed or logged in plain text.
- **Automated Cluster Validation**: Multi-stage health checks for nodes, pods, readiness probes, endpoints, and HTTP health URLs.
- **Cryptographic Approval Gate**: Cutover requires explicit human authorization, sealed with deterministic SHA-256 plan and validation hashes.
- **Instant Rollback**: Emergency `recover` command to immediately route traffic back to the original Blue cluster if needed.
- **Mock Mode for Offline Testing**: Full-featured mock AWS and Kubernetes client implementations allowing local testing without active AWS credentials.

---

## 🛠️ CLI Command Reference

| Command | Usage | Description |
|---|---|---|
| `init` | `eksbg init` | Generates initial config file and checks prerequisite tools (`aws`, `kubectl`, `helm`) |
| `inspect` | `eksbg inspect -c <cluster> -r <region>` | Inspects EKS cluster, node groups, add-ons, and K8s workloads; outputs Markdown/JSON report |
| `plan` | `eksbg plan -s <blue> --target-version 1.30` | Compares cluster configurations, checks compatibility, and generates `./plans/upgrade.yaml` |
| `apply` | `eksbg apply -p ./plans/upgrade.yaml [--dry-run]` | Provisions the Green EKS control plane and node groups based on the plan |
| `sync` | `eksbg sync --job <job-id>` | Deploys workloads and configurations to the Green cluster |
| `validate` | `eksbg validate --job <job-id>` | Executes automated health and readiness checks (PASS/WARN/FAIL) |
| `approve-cutover` | `eksbg approve-cutover --job <job-id>` | Displays validation findings, endpoint differences, and prompts for explicit user confirmation |
| `cutover` | `eksbg cutover --job <job-id>` | Validates approval integrity and switches traffic to Green cluster via Route53 / ALB |
| `recover` | `eksbg recover --job <job-id>` | Emergency rollback of production traffic back to the Blue cluster |
| `status` | `eksbg status [--job <job-id>]` | Displays current phase, status, and step-by-step audit logs |
| `report` | `eksbg report --job <job-id> [-f markdown\|json]` | Generates a comprehensive audit report for compliance and post-mortems |

---

## 📦 Quick Start & Demo Flow

### 1. Build the Binary
```bash
go build -o ./eksbg ./cmd/eksbg
```

### 2. Initialize Configuration
```bash
./eksbg init
```

### 3. Inspect Source Cluster
```bash
# Using mock mode (for local offline testing):
./eksbg inspect --cluster blue-cluster --mock

# Against real AWS:
./eksbg inspect --cluster prod-cluster --region ap-northeast-2
```

### 4. Generate Upgrade Plan
```bash
./eksbg plan --source blue-cluster --target-version 1.30 --mock
```

### 5. Provision Green Infrastructure (Dry-run & Apply)
```bash
# Dry-run test:
./eksbg apply --plan ./plans/upgrade.yaml --dry-run --mock

# Actual provisioning:
./eksbg apply --plan ./plans/upgrade.yaml --mock
```

### 6. Synchronize Workloads
```bash
./eksbg sync --job <job-id> --mock
```

### 7. Automated Cluster Validation
```bash
./eksbg validate --job <job-id> --mock
```

### 8. Explicit Cutover Approval
```bash
./eksbg approve-cutover --job <job-id> --approver "sre-lead" --yes --mock
```

### 9. Traffic Cutover
```bash
./eksbg cutover --job <job-id> --mock
```

### 10. Emergency Rollback (If needed)
```bash
./eksbg recover --job <job-id> --yes --mock
```

---

## 🧪 Test Results

All 12 required test specifications are fully implemented and passing:

```text
=== RUN   TestCutoverBlockedWithoutApproval       --- PASS: (0.01s)
=== RUN   TestCutoverBlockedOnValidationFailure   --- PASS: (0.00s)
=== RUN   TestPlanTamperingInvalidatesApproval    --- PASS: (0.00s)
=== RUN   TestPartialFailureAndRecovery           --- PASS: (0.00s)
=== RUN   TestDryRunDoesNotMutateResources        --- PASS: (0.00s)
=== RUN   TestEndToEndLifecycleWorkflow           --- PASS: (0.01s)
=== RUN   TestAWSMockOperations                   --- PASS: (0.00s)
=== RUN   TestConfigLoadingAndDefaults            --- PASS: (0.00s)
=== RUN   TestConfigEnvironmentOverrides          --- PASS: (0.00s)
=== RUN   TestKubernetesResourceInspection        --- PASS: (0.00s)
=== RUN   TestPlanGenerationAndCompatibility      --- PASS: (0.00s)
=== RUN   TestSecretMasking                       --- PASS: (0.00s)
=== RUN   TestBlueClusterProtectionGuardrails     --- PASS: (0.00s)
=== RUN   TestDeterministicPlanHash               --- PASS: (0.00s)
=== RUN   TestStoreIdempotencyAndJobResume        --- PASS: (0.00s)
PASS
ok  	eks-bluegreen-cli/tests	0.407s
```

Run tests at any time with:
```bash
go test -v ./...
```

---

## 🔒 Safety & Guardrail Architecture

Refer to [docs/architecture.md](docs/architecture.md) and [docs/security.md](docs/security.md) for full architectural specifications, including:
- Finite State Machine transitions (`INIT` -> `INSPECTED` -> `PLANNED` -> `PROVISIONED` -> `SYNCED` -> `VALIDATED` -> `APPROVED` -> `CUTOVER_COMPLETED`)
- Least-privilege IAM policy without `eks:DeleteCluster` permissions.
- In-memory SQLite state persistence with WAL journaling.

---

## ⚠️ Known Limitations & Future Roadmap

1. **StatefulSet & Database Data Synchronization**: `eksbg` detects StatefulSets and persistent volumes and warns the operator. Automated live replication of stateful volumes or databases (e.g. Amazon RDS multi-AZ failover) requires custom application-level migration strategies.
2. **ArgoCD / GitOps Native Sync**: Currently replicates workloads via Kubernetes manifests; a dedicated GitOps provider plugin to update repository deployment branches directly is planned for Phase 7.
3. **Multi-Cluster Mesh Traffic Shifting**: AWS Route53 weighted records and ALB target groups are supported; Istio / Envoy multi-cluster mesh traffic splitting will be added in upcoming releases.
