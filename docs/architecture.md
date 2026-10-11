# Architecture & Design: EKS Blue-Green Upgrade CLI (`eksbg`)

## 1. Overview

`eksbg` is a modular, production-grade CLI built to eliminate downtime and operational risks during Amazon EKS Kubernetes version upgrades. Instead of performing in-place upgrades that can cause control plane unavailability or node disruptions, `eksbg` orchestrates an automated Blue-Green migration pattern.

```
                    Traffic Ingress (Route53 / ALB)
                                   |
                +------------------+------------------+
                | (Before Cutover: 100%)              | (After Cutover: 100%)
                v                                     v
       +------------------+                  +------------------+
       |   Blue Cluster   |                  |  Green Cluster   |
       |  EKS v1.28 (RO)  |                  |    EKS v1.30     |
       +------------------+                  +------------------+
       | Read-only        |                  | Provisioned      |
       | Kept Intact      |                  | Validated        |
       | Rollback Target  |                  | Workloads Synced |
       +------------------+                  +------------------+
```

---

## 2. Core Operational Principles

1. **Blue Cluster Read-Only Guardrail**: The existing Blue cluster is strictly treated as read-only. Destructive modifications or automated deletions of Blue resources are blocked by software guardrails.
2. **Automated Green Lifecycle**: Green cluster provisioning, nodegroup creation, addon setup, and workload deployment are automated and fully resumable.
3. **Cryptographic Approval Gate**: Cutover requires explicit human approval with verification of plan and validation SHA-256 hashes. If the plan or validation changes after approval, the approval is invalidated.
4. **Resumable State Machine**: All jobs, execution steps, and approval records are tracked in a SQLite state store. Interrupted steps can be resumed seamlessly.
5. **Safe Secret Handling**: Sensitive credentials, tokens, and AWS access keys are never dumped in cleartext and are masked automatically in logs and reports.

---

## 3. Workflow State Machine

The workflow progresses through distinct phases:

```
INIT
  │
  ▼
INSPECTED
  │
  ▼
PLANNED
  │
  ▼
PROVISIONING ──(Retry/Resume)──► PROVISIONED
  │
  ▼
SYNCING ──────(Retry/Resume)──► SYNCED
  │
  ▼
VALIDATING ───(Re-check)──────► VALIDATED
  │
  ▼
WAITING_FOR_APPROVAL
  │
  ▼
APPROVED (Plan & Validation Hash Verified)
  │
  ▼
CUTOVER_IN_PROGRESS ─────────► CUTOVER_COMPLETED
                                      │
                                      ▼ (Emergency Rollback)
                                  RECOVERED
```

---

## 4. Module Boundaries

- **`cmd/eksbg`**: Command line entrypoint.
- **`internal/cli`**: Cobra CLI commands with flag parsing, validation, and user interaction.
- **`internal/config`**: Viper configuration management with YAML persistence and environment variable support.
- **`internal/aws`**: AWS SDK v2 client interface abstraction and complete in-memory mock client.
- **`internal/inspector`**: Cluster metadata, node group, addon, and deprecated API scanning.
- **`internal/planner`**: Resource classification, target version compatibility, cost estimation, and plan hash generation.
- **`internal/provisioner`**: Green EKS control plane and node group provisioning with dry-run support.
- **`internal/deployer`**: Kubernetes resource synchronization with safe secret references and storage checks.
- **`internal/validator`**: Health, node status, probe readiness, and HTTP endpoint verification.
- **`internal/approval`**: Cutover approval manager with hash integrity checks.
- **`internal/cutover`**: Route53 / ALB traffic switching with Blue cluster protection.
- **`internal/recovery`**: Instant traffic rollback back to Blue cluster.
- **`internal/store`**: Pure-Go SQLite persistence for jobs, steps, plans, approvals, and validation results.
- **`internal/security`**: Credential masking, guardrails, and cryptographic hashing.
- **`internal/reporter`**: Markdown and JSON audit report generation.
