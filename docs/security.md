# Security & Guardrails Guide: `eksbg`

## 1. Built-in Security Guardrails

| Guardrail | Purpose | Enforcement Point |
|---|---|---|
| **Blue Cluster Preservation** | Prevents deletion or destructive modification of the active Blue cluster | `internal/security/AssertSafeOperation`, `internal/cutover/ProtectBlueClusterStrict` |
| **Explicit Cutover Approval** | Blocks traffic switching unless explicit user confirmation is recorded | `internal/approval/VerifyApproval` |
| **Hash Tampering Detection** | Invalidates approvals if plan YAML or validation checks are altered | `security.ComputePlanHash`, `approval.VerifyApproval` |
| **Critical Validation Gate** | Blocks traffic switching if critical checks (e.g. pods not ready, cluster down) fail | `validator.HasCriticalFailures` |
| **Secret Masking** | Prevents AWS keys, bearer tokens, passwords, and secrets from appearing in logs | `security.MaskSensitiveData` |
| **Safe Process Execution** | External commands run with argument slices, preventing shell injection | `security.SafeExec` |

---

## 2. Minimum Required AWS IAM Policy

To run `eksbg` with least privilege in AWS, attach the following IAM policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EKSClusterInspectionAndProvisioning",
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListClusters",
        "eks:CreateCluster",
        "eks:DescribeNodegroup",
        "eks:ListNodegroups",
        "eks:CreateNodegroup",
        "eks:DescribeAddon",
        "eks:ListAddons",
        "eks:CreateAddon",
        "eks:DescribeAddonVersions"
      ],
      "Resource": "*"
    },
    {
      "Sid": "EC2NetworkInspection",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeVpcs",
        "ec2:DescribeSubnets",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeRouteTables"
      ],
      "Resource": "*"
    },
    {
      "Sid": "Route53CutoverManagement",
      "Effect": "Allow",
      "Action": [
        "route53:ListResourceRecordSets",
        "route53:ChangeResourceRecordSets"
      ],
      "Resource": "arn:aws:route53:::hostedzone/*"
    },
    {
      "Sid": "STSCallerIdentity",
      "Effect": "Allow",
      "Action": [
        "sts:GetCallerIdentity"
      ],
      "Resource": "*"
    }
  ]
}
```

> **Note**: `eks:DeleteCluster` is intentionally omitted from the recommended policy to provide infrastructure-level protection against accidental Blue cluster deletion.
