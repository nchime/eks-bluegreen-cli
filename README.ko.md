# EKS Blue-Green 업그레이드 CLI (`eksbg`)

[English](README.md) | [한국어](README.ko.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org)
[![Tests](https://img.shields.io/badge/Tests-15%20passed-brightgreen.svg)](#-테스트-결과)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

`eksbg`는 **Amazon EKS Kubernetes 버전 업그레이드** 시 발생하는 다운타임, 컨트롤 플레인 성능 저하 및 운영 위험을 최소화하기 위해 설계된 오픈소스 수준의 프로덕션 등급 CLI 도구입니다.

기존 EKS 클러스터(Blue)를 분석하고, 목표 Kubernetes 버전의 새로운 클러스터(Green)를 생성하며, 안전 가드레일에 따라 구성과 워크로드를 복제하고, 클러스터 준비 상태를 자동 검증한 뒤 사용자의 명시적인 승인에 따라 트래픽을 전환합니다.

---

## 🚀 주요 기능 (Key Features)

- **Blue 클러스터 읽기 전용 가드레일 (Read-Only Blue)**: 기존 Blue 클러스터는 읽기 전용으로 보호되며, 자동 삭제나 파괴적인 변경 작업이 원천 차단됩니다.
- **자동화된 Green 라이프사이클**: Green EKS 컨트롤 플레인, Managed Node Group, Add-on 생성을 자동화하며 장애 발생 시 중단된 지점부터 안전하게 재개(Resumable)할 수 있습니다.
- **API Deprecation 자동 감지**: 목표 Kubernetes 버전에서 제거되거나 변경된 API(v1beta1 등)를 사용하는 워크로드를 사전에 식별합니다.
- **민감 정보(Secret) 보호**: Secret 값을 평문으로 로깅하거나 출력하지 않으며, 안전한 외부 참조 방식으로 안전하게 처리합니다.
- **포괄적인 자동 검증 (Automated Validation)**: 노드 준비 상태, 파드 Readiness 프로브, 서비스 엔드포인트, Ingress 및 HTTP 헬스체크를 종합 검증(PASS/WARN/FAIL)합니다.
- **암호학적 승인 게이트 (Cryptographic Approval Gate)**: 트래픽 전환 전 Plan과 검증 결과의 SHA-256 해시를 검증하며, 변경 사항 발생 시 기존 승인이 자동으로 무효화됩니다.
- **긴급 즉시 롤백 (Instant Rollback)**: 전환 후 문제 발생 시 `recover` 명령으로 트래픽을 기존 Blue 클러스터로 즉시 복원할 수 있습니다.
- **오프라인/로컬 테스트 지원 (Mock Mode)**: AWS 인증 정보 없이도 동작하는 완벽한 Mock AWS 및 K8s 클라이언트를 내장하여 로컬 개발 환경에서 전체 생명주기를 시연할 수 있습니다.

---

## 🛠️ CLI 명령어 명세 (Command Reference)

| 명령어 | 사용 예시 | 설명 |
|---|---|---|
| `init` | `eksbg init` | 기본 설정 파일 생성 및 필수 도구(`aws`, `kubectl`, `helm`, AWS 인증) 점검 |
| `inspect` | `eksbg inspect --cluster <name> -r <region>` | EKS 클러스터, 노드그룹, Add-on, K8s 워크로드 및 Deprecated API 분석 보고서 출력 |
| `plan` | `eksbg plan -s <blue> --target-version 1.30` | 버전 호환성 검사, 서브넷 IP 용량 확인, 리소스 분류, 비용 산출 및 `./plans/upgrade.yaml` 생성 |
| `apply` | `eksbg apply -p ./plans/upgrade.yaml [--dry-run]` | Plan에 따라 Green EKS 컨트롤 플레인 및 노드그룹 프로비저닝 (Dry-run 지원) |
| `sync` | `eksbg sync --job <job-id>` | 워크로드, 네임스페이스, 서비스를 Green 클러스터로 배포하고 스토리지 마이그레이션 요구사항 확인 |
| `validate` | `eksbg validate --job <job-id>` | 노드 상태, 파드 프로브, 엔드포인트 및 HTTP 헬스체크 자동 검증 수행 |
| `approve-cutover` | `eksbg approve-cutover --job <job-id>` | 검증 결과 및 Blue/Green 엔드포인트 비교 확인 후 명시적 사용자 승인 기록 |
| `cutover` | `eksbg cutover --job <job-id>` | 승인 무결성을 검증하고 Route53 DNS / ALB 트래픽을 Green 클러스터로 전환 (**Blue는 보존**) |
| `recover` | `eksbg recover --job <job-id>` | 비상 상황 시 운영 트래픽을 기존 Blue 클러스터로 즉시 복귀 |
| `status` | `eksbg status [--job <job-id>]` | 작업 진행 단계, 상태 및 세부 실행 이력(Audit Log) 조회 |
| `report` | `eksbg report --job <job-id> [-f markdown\|json]` | 감사 및 규정 준수를 위한 종합 결과 보고서 생성 |

---

## 📦 빠른 시작 및 데모 실행 가이드

### 1. 바이너리 빌드
```bash
go build -o ./eksbg ./cmd/eksbg
```

### 2. 설정 초기화
```bash
./eksbg init
```

### 3. 소스 클러스터 조사 (Inspect)
```bash
# 로컬 오프라인 Mock 모드 실행:
./eksbg inspect --cluster blue-cluster --mock

# 실제 AWS 환경 대상 실행:
./eksbg inspect --cluster prod-cluster --region ap-northeast-2
```

### 4. 업그레이드 계획 수립 (Plan)
```bash
./eksbg plan --source blue-cluster --target-version 1.30 --mock
```

### 5. Green 인프라 프로비저닝 (Dry-run 및 Apply)
```bash
# Dry-run 시뮬레이션:
./eksbg apply --plan ./plans/upgrade.yaml --dry-run --mock

# 실제 프로비저닝 실행:
./eksbg apply --plan ./plans/upgrade.yaml --mock
```

### 6. 워크로드 동기화 (Sync)
```bash
./eksbg sync --job <job-id> --mock
```

### 7. 자동 클러스터 검증 (Validate)
```bash
./eksbg validate --job <job-id> --mock
```

### 8. 명시적 트래픽 전환 승인 (Approve)
```bash
./eksbg approve-cutover --job <job-id> --approver "sre-lead" --yes --mock
```

### 9. 트래픽 전환 (Cutover)
```bash
./eksbg cutover --job <job-id> --mock
```

### 10. 긴급 롤백 (필요 시)
```bash
./eksbg recover --job <job-id> --yes --mock
```

---

## 🧪 테스트 결과

Section 10의 12대 핵심 요구사항 및 전체 수명주기 테스트가 구현되어 모두 통과했습니다:

```text
=== RUN   TestCutoverBlockedWithoutApproval       --- PASS: (0.01s) # [6] 승인 없이 cutover 실행 시 차단
=== RUN   TestCutoverBlockedOnValidationFailure   --- PASS: (0.00s) # [7] 검증 실패 시 cutover 차단
=== RUN   TestPlanTamperingInvalidatesApproval    --- PASS: (0.00s) # [8] Plan 변경 시 기존 승인 무효화
=== RUN   TestPartialFailureAndRecovery           --- PASS: (0.00s) # [11] 부분 실패 후 복구
=== RUN   TestDryRunDoesNotMutateResources        --- PASS: (0.00s) # [12] Dry-run 리소스 미변경 검증
=== RUN   TestEndToEndLifecycleWorkflow           --- PASS: (0.01s) # 전체 라이프사이클 E2E 연동
=== RUN   TestAWSMockOperations                   --- PASS: (0.00s) # [2] AWS API Mock 테스트
=== RUN   TestConfigLoadingAndDefaults            --- PASS: (0.00s) # [1] CLI 명령어 및 설정 파싱
=== RUN   TestConfigEnvironmentOverrides          --- PASS: (0.00s) # [1] 환경변수 오버라이드
=== RUN   TestKubernetesResourceInspection        --- PASS: (0.00s) # [3] Kubernetes 리소스 및 Deprecated API 분석
=== RUN   TestPlanGenerationAndCompatibility      --- PASS: (0.00s) # [4] Plan 생성 및 호환성 검사
=== RUN   TestSecretMasking                       --- PASS: (0.00s) # [10] Secret 마스킹 테스트
=== RUN   TestBlueClusterProtectionGuardrails     --- PASS: (0.00s) # [9] Blue 클러스터 삭제/변경 방지
=== RUN   TestDeterministicPlanHash               --- PASS: (0.00s) # 결정론적 Plan 해시 검증
=== RUN   TestStoreIdempotencyAndJobResume        --- PASS: (0.00s) # [5] 멱등성 및 작업 재개 테스트
PASS
ok  	eks-bluegreen-cli/tests	0.434s
```

테스트 실행 명령어:
```bash
go test -v ./...
```

---

## 🔒 안전성 및 아키텍처 가이드

세부 아키텍처 및 보안 설계 문서는 다음 경로에서 확인하실 수 있습니다:
- [docs/architecture.md](docs/architecture.md): 유한 상태 머신 (FSM: `INIT` -> `INSPECTED` -> `PLANNED` -> `PROVISIONED` -> `SYNCED` -> `VALIDATED` -> `APPROVED` -> `CUTOVER_COMPLETED`), 모듈 경계, SQLite 상태 저장소 설계
- [docs/security.md](docs/security.md): 최소 권한 IAM 정책 문서 (`eks:DeleteCluster` 권한 제외), 보안 가드레일, Secret 마스킹 정책

---

## ⚠️ 알려진 제한사항 및 향후 로드맵

1. **StatefulSet 및 데이터베이스 데이터 동기화**: `eksbg`는 StatefulSet과 영구 볼륨(PVC)을 감지하고 운영자에게 수동 마이그레이션 필요성을 경고합니다. 데이터베이스(예: Amazon RDS) 및 영구 스토리지의 실시간 양방향 복제는 애플리케이션 특성에 맞는 별도 데이터 마이그레이션 전략이 필요합니다.
2. **Argo CD / GitOps 네이티브 연동**: 현재는 Kubernetes 매니페스트 기반으로 워크로드를 동기화합니다. 추후 버전에서 배포 브랜치나 Git 저장소를 직접 갱신하는 전용 GitOps 플러그인이 추가될 예정입니다.
3. **서비스 메시 기반 세밀한 트래픽 분할**: 현재 Route53 가중치 DNS 및 ALB 기반 트래픽 전환을 지원하며, 향후 Istio / Envoy 기반의 카나리 퍼센티지 트래픽 스플릿 기능이 확장될 예정입니다.
