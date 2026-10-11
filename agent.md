# Project: EKS Blue-Green Upgrade CLI

## 1. 역할 및 목표

당신은 AWS EKS, Kubernetes, Go 기반 인프라 자동화 및 SRE 시스템 구축 경험이 풍부한 시니어 플랫폼 엔지니어이자 소프트웨어 아키텍트다.

`eks-bluegreen-cli`라는 이름의 오픈소스 수준의 CLI 프로그램을 개발하라.

이 프로그램은 기존 AWS EKS 클러스터를 분석하고, 목표 Kubernetes 버전으로 새로운 Green 클러스터를 생성하며, 기존 클러스터의 구성과 워크로드를 복제하고, 검증을 완료한 후 사용자의 명시적인 승인에 따라 트래픽을 전환할 수 있도록 지원한다.

핵심 목표는 EKS Kubernetes 버전 업그레이드 과정에서 수작업과 운영 위험을 줄이는 것이다.

단순히 코드를 생성하는 데 그치지 말고, 실제 AWS 환경에서 실행할 수 있는 구조와 테스트를 갖춘 MVP를 구현하라.

## 2. 핵심 요구사항

### 2.1 운영 환경

- 클라우드: AWS
- Kubernetes: Amazon EKS
- 언어: Go
- CLI 프레임워크: Cobra
- 설정 관리: Viper
- AWS 연동: AWS SDK for Go v2
- Kubernetes 연동: client-go, discovery API
- 패키지 배포: 단일 Go 실행 파일
- 설정 형식: YAML 및 JSON
- 로컬 작업 상태 저장: SQLite
- 선택적 원격 상태 저장: Amazon S3
- 인프라 생성: AWS SDK/API 또는 AWS CLI 연동
- 워크로드 배포: kubectl, Helm, 선택적 Argo CD 연동
- 테스트: Go testing, mock 기반 단위 테스트, 통합 테스트

AWS 계정, 리전, IAM 권한 및 kubectl context를 명시적으로 확인하라.

### 2.2 운영 원칙

1. 기존 Blue 클러스터는 기본적으로 읽기 전용으로 취급한다.
2. Green 클러스터의 생성과 검증은 자동화한다.
3. 트래픽 전환은 반드시 사용자의 명시적인 승인 후에만 실행한다.
4. Blue 클러스터를 자동 삭제하거나 기존 리소스를 파괴하지 않는다.
5. 모든 작업은 재실행할 수 있어야 하며 중간 실패 후 재개할 수 있어야 한다.
6. 모든 변경 작업을 실행하기 전에 Plan과 예상 변경사항을 제공한다.
7. 자격 증명, Secret 값, 토큰을 로그나 보고서에 출력하지 않는다.
8. 실제 운영 작업과 Dry-run을 명확히 구분한다.
9. 파괴적 작업은 별도의 명시적 승인과 보호 절차를 요구한다.
10. 실행 결과와 변경 이력을 감사 로그로 남긴다.

## 3. CLI 명령어 설계

다음 명령어를 구현하라.

### init

`eksbg init`

- 초기 설정 파일 생성
- AWS Profile, Region, 작업 디렉터리 설정
- 외부 명령어 설치 여부 검사
- 필수 권한 및 도구 검사

### inspect

`eksbg inspect --cluster blue-cluster --region ap-northeast-2`

- 현재 EKS 버전 및 클러스터 정보 조회
- VPC, 서브넷, 보안 그룹, 엔드포인트 정보 조회
- Managed Node Group 및 Fargate 설정 조회
- EKS Add-ons, IAM Role, OIDC Provider 정보 조회
- Kubernetes API 리소스 및 워크로드 분석
- StorageClass, PVC, PV, Ingress, Service 분석
- Helm 릴리스 및 선택적 GitOps 설정 분석
- 버전 업그레이드 호환성 및 잠재적 문제 식별
- JSON/YAML 및 Markdown 보고서 생성

민감한 Secret 값은 수집하거나 출력하지 않는다.

### plan

`eksbg plan --source blue-cluster --target-version 1.xx --region ap-northeast-2`

- 소스와 목표 클러스터 구성 비교
- 목표 Kubernetes 버전 지원 여부 확인
- Add-on 호환성 검사
- API deprecation 및 제거된 API 검사
- 노드 AMI 및 아키텍처 호환성 검토
- 서브넷 IP, 용량, 인스턴스 타입 검토
- 복제할 리소스와 제외할 리소스 분류
- 예상 비용 및 생성 리소스 산출
- 사전 조건 및 차단 사유 보고

`1.xx`는 예시다. 실제 지원 버전을 AWS API에서 조회하며 하드코딩하지 않는다.

### apply

`eksbg apply --plan ./plans/upgrade.yaml`

- Plan 파일 검증
- 생성할 리소스 목록과 변경사항 출력
- Dry-run 지원
- 승인된 계획을 기준으로 Green 환경 생성
- 생성 결과를 상태 저장소에 기록
- 작업 실패 시 원인과 재개 가능한 지점 표시

### sync

`eksbg sync --job <job-id>`

- 승인된 복제 계획에 따라 워크로드 배포
- Helm 릴리스 재설치 또는 선언적 매니페스트 적용
- Secret은 값이 아닌 안전한 참조 방식으로 처리
- Storage 및 데이터 마이그레이션 요구사항 확인
- 필요한 경우 애플리케이션별 마이그레이션 단계 실행
- 재실행 시 중복 생성과 불필요한 변경 방지

### validate

`eksbg validate --job <job-id>`

다음 항목을 검증한다.

- Green 클러스터 및 노드 상태
- 시스템 및 필수 Add-on 상태
- Deployment, Pod, DaemonSet, StatefulSet 상태
- Readiness/Liveness Probe
- Service 및 Endpoint 연결
- Ingress 및 ALB 연결 상태
- DNS, 네트워크, IAM 권한
- PVC 및 Storage 연결
- 애플리케이션 HTTP 헬스체크
- 선택적 부하 테스트와 기준 성능 비교

검증 결과를 PASS, WARN, FAIL로 분류하고, 필수 검증에 실패하면 트래픽 전환을 차단한다.

### approve-cutover

`eksbg approve-cutover --job <job-id>`

- 현재 검증 결과 표시
- Blue/Green 엔드포인트 비교
- 위험 요소 및 남은 경고 표시
- 사용자에게 명시적인 확인 입력 요구
- 승인자, 시각, 검증 결과를 기록
- 승인 시에만 전환 단계 활성화

### cutover

`eksbg cutover --job <job-id>`

- 유효한 승인 기록 확인
- 승인 후 변경된 환경이 없는지 재검증
- 사전에 설정된 트래픽 전환 방식을 실행
- DNS, Ingress, ALB 또는 서비스 라우팅 변경
- 단계별 상태 확인 및 감사 로그 기록
- 전환 실패 시 기존 Blue 환경 유지
- 기존 환경의 자동 삭제 금지

트래픽 전환 방식은 사전에 설정하고 검증한다. DNS 전환 시 캐시 및 TTL을 고려하며, ALB 전환 시 실제 Listener/Target Group 구성과 라우팅 정책을 확인한다.

### recover

`eksbg recover --job <job-id>`

- 현재 트래픽 경로 및 클러스터 상태 확인
- 기존 Blue 환경으로 복귀할 수 있는지 검사
- 롤백 영향 및 데이터 일관성 경고 표시
- 사용자 승인 후 지원되는 방식으로 트래픽 복귀
- 복귀 결과 검증 및 감사 로그 기록

데이터베이스 스키마 변경이나 양방향 데이터 동기화는 자동으로 되돌릴 수 있다고 가정하지 않는다.

### status 및 report

`eksbg status --job <job-id>`

`eksbg report --job <job-id> --format markdown`

- 전체 진행 상태 및 단계별 결과
- 실패 원인 및 재개 가능한 작업
- Green/Blue 구성 비교
- 검증 결과
- 승인 및 전환 이력
- Markdown, JSON 보고서 생성

## 4. 아키텍처 및 모듈 구조

단일 Go 바이너리 기반의 모듈형 모놀리스를 구성한다.

권장 디렉터리 구조:

cmd/
  eksbg/
internal/
  cli/
  config/
  aws/
  inspector/
  planner/
  provisioner/
  deployer/
  validator/
  workflow/
  approval/
  cutover/
  recovery/
  store/
  reporter/
  security/
pkg/
  models/
  errors/
configs/
plans/
templates/
tests/
docs/

각 모듈의 책임을 분리하고 AWS SDK, Kubernetes API, 외부 명령어 실행을 인터페이스로 추상화한다.

### 핵심 모듈

- Inspector: 소스 클러스터 및 AWS 리소스 조사
- Planner: 복제 계획 및 호환성 검사
- Provisioner: Green 인프라 생성
- Deployer: Kubernetes 리소스 및 Helm 릴리스 배포
- Validator: 자동 검증
- Workflow: 작업 상태와 실행 순서 관리
- Approval: 수동 승인 및 승인 이력 관리
- Cutover: 트래픽 전환
- Recovery: 복구 절차
- Store: SQLite/S3 기반 작업 상태 저장
- Reporter: 결과 보고서 생성
- Security: 권한 검사, Secret 보호, 실행 가드레일

## 5. 인프라 복제 전략

모든 AWS 리소스를 무조건 복사하지 말고 리소스 유형별로 복제 정책을 구분하라.

### 복제 및 재구성 대상

- EKS Control Plane
- Managed Node Group 및 Launch Template
- VPC, Subnet, Security Group, Route Table 등 네트워크 설정
- EKS Add-ons
- IAM Role 및 OIDC 연동
- Kubernetes Namespace, RBAC, ConfigMap
- Deployment, StatefulSet, DaemonSet, Service
- Ingress 및 관련 로드밸런서 설정
- Helm 릴리스 및 애플리케이션 설정
- StorageClass 및 영구 스토리지 연결 설정
- HPA, PDB 및 관련 스케일링 설정

### 특별 취급 대상

- Kubernetes Secret: 값을 로그에 남기지 않고 안전하게 재구성
- PVC/PV 및 EBS: 기존 볼륨을 무분별하게 재사용하지 않으며 스냅샷, 복원, 데이터 복제 전략을 별도 정의
- RDS 및 외부 데이터베이스: 데이터 일관성과 쓰기 전환 전략 필요
- ALB/NLB: 기존 로드밸런서와 충돌하지 않도록 Green 전용 리소스 구성
- IAM 및 보안 그룹: 권한 최소화 및 참조 관계 검증
- Argo CD: Git 선언형 구성을 우선 활용하고 관리 주체의 중복을 방지
- 클러스터별 리소스: 기존 클러스터 UID, IP, ARN 등 고유 식별자를 그대로 복사하지 않음

복제할 수 없는 리소스는 조용히 건너뛰지 말고 이유와 수동 조치 방법을 보고한다.

## 6. 작업 상태 및 데이터 모델

다음 모델을 구현한다.

### UpgradeJob

- ID
- SourceCluster
- SourceRegion
- TargetCluster
- TargetRegion
- SourceVersion
- TargetVersion
- CurrentPhase
- Status
- CreatedAt
- UpdatedAt
- LastError

### ResourcePlan

- JobID
- ResourceType
- ResourceName
- Action
- Dependencies
- RiskLevel
- ValidationStatus
- Notes

### ApprovalRecord

- JobID
- ApprovalType
- Approver
- ApprovedAt
- PlanHash
- ValidationHash
- Status

### ExecutionStep

- JobID
- StepName
- Status
- StartedAt
- CompletedAt
- RetryCount
- ErrorMessage
- OutputReference

### ValidationResult

- JobID
- CheckName
- Severity
- Status
- Details
- CheckedAt

민감한 값은 데이터 모델이나 로그에 평문으로 저장하지 않는다. SQLite를 기본 저장소로 사용하며, S3 저장 시 암호화 및 접근 제어를 적용한다.

## 7. 워크플로 엔진

상태 머신 기반으로 구현한다.

INIT
→ INSPECTED
→ PLANNED
→ PROVISIONING
→ PROVISIONED
→ SYNCING
→ SYNCED
→ VALIDATING
→ VALIDATED
→ WAITING_FOR_APPROVAL
→ APPROVED
→ CUTOVER_IN_PROGRESS
→ CUTOVER_COMPLETED

오류 발생 시 FAILED 또는 RECOVERABLE 상태를 기록한다.

요구사항:

- 단계별 재실행 가능
- 멱등성 보장
- 단계별 타임아웃 및 재시도 정책
- 작업 잠금 및 동시 실행 방지
- 의존성 순서 보장
- 작업 중단 후 재개
- 실패 단계와 복구 방법 안내
- 승인 이후 Plan 또는 검증 상태가 변경되면 재승인 요구

## 8. 안전성 및 보안

다음 규칙은 필수다.

- 기본 모드는 조회 또는 Dry-run
- Blue 클러스터 변경 및 삭제 금지
- 자동 트래픽 전환 금지
- 자동 Blue 클러스터 삭제 금지
- 실제 변경 전 Plan 확인
- 대상 AWS 계정 및 리전 확인
- 최소 권한 IAM 정책 문서 제공
- 실행 로그에서 Secret, Access Key, Token 마스킹
- 외부 명령어 실행 시 셸 문자열 결합 대신 인자 배열 사용
- 모든 AWS API 작업의 실패 및 재시도 처리
- 중복 실행 및 부분 실패 보호
- 승인된 계획과 실행 계획의 해시 비교
- 데이터 손실 위험이 있는 단계는 별도 차단
- 롤백 가능 여부와 한계 명시

## 9. 설정 파일

YAML 기반 설정 예시를 제공한다.

설정 항목:

- AWS Profile 및 Region
- Blue 클러스터 이름
- Green 클러스터 이름
- 목표 Kubernetes 버전
- VPC/Subnet 구성
- Node Group 및 인스턴스 타입
- Add-on 버전 정책
- 복제 및 제외할 리소스
- Helm/GitOps 설정
- 헬스체크 URL 및 타임아웃
- 트래픽 전환 방식
- 수동 승인 정책
- 작업 상태 저장 방식
- 보고서 출력 경로

비밀 정보는 설정 파일에 직접 저장하지 말고 AWS Secrets Manager, SSM Parameter Store 또는 IAM Role을 활용한다.

## 10. 테스트 및 완료 기준

다음 테스트를 반드시 작성한다.

1. CLI 명령어 및 설정 파싱 단위 테스트
2. AWS API Mock 테스트
3. Kubernetes 리소스 분석 테스트
4. Plan 생성 및 호환성 검사 테스트
5. 멱등성 및 작업 재개 테스트
6. 승인 없이 cutover 실행 시 차단되는 테스트
7. 검증 실패 시 cutover가 차단되는 테스트
8. Plan 변경 시 기존 승인 무효화 테스트
9. Blue 클러스터 삭제 방지 테스트
10. Secret 마스킹 테스트
11. 부분 실패 후 복구 테스트
12. Dry-run이 실제 리소스를 변경하지 않는 테스트

실제 AWS 환경에 접근하지 않아도 핵심 로직의 단위 테스트가 통과해야 한다.

통합 테스트는 별도의 AWS 테스트 계정에서 실행하도록 분리한다.

## 11. 개발 진행 방식

한 번에 모든 기능을 구현하지 말고 다음 단계로 진행한다.

Phase 1: 프로젝트 골격
- Go 모듈 및 Cobra CLI 생성
- Viper 설정 관리
- 기본 명령어와 도움말
- 로그 및 에러 처리
- CI 및 테스트 구성

Phase 2: Read-only Inspector
- AWS 인증 및 권한 확인
- EKS 및 Kubernetes 리소스 조회
- JSON/Markdown 보고서 생성

Phase 3: Planner
- 리소스 분류
- 목표 버전 및 Add-on 호환성 검사
- 복제 계획 생성
- Dry-run 및 Plan 검토

Phase 4: Provisioner
- Green 클러스터 생성
- 노드 그룹 및 Add-on 구성
- 네트워크 및 IAM 검증
- 실패 후 작업 재개

Phase 5: Deployer 및 Validator
- 워크로드 배포
- Helm 및 GitOps 연동
- 헬스체크 및 리소스 검증

Phase 6: Approval 및 Cutover
- 승인 기록
- 트래픽 전환
- 전환 상태 확인
- 복구 기능

Phase 7: 운영 안정화
- 통합 테스트
- 문서 및 샘플 설정
- 실행 보고서
- 보안 점검
- 릴리스 및 설치 문서

각 Phase가 끝날 때마다 코드, 테스트, README, 남은 작업을 점검하고 다음 Phase로 진행하라.

## 12. 최초 실행 지침

먼저 프로젝트 구조, 모듈 경계, 상태 머신, 데이터 모델, CLI 명령어 명세를 작성하라.

그다음 Phase 1을 실제 코드로 구현하고 테스트를 실행하라.

AWS 자격 증명이 없는 개발 환경에서는 Mock 기반으로 구현하라. 실제 AWS 리소스가 존재한다고 가정하거나 성공 결과를 조작하지 말라.

실행 가능한 명령어, 테스트 결과, 미구현 기능, 알려진 제한사항을 README에 기록하라.

핵심 목표는 데모용 코드가 아니라 안전하고 재실행 가능하며 실제 운영 환경으로 확장할 수 있는 EKS Blue-Green 업그레이드 자동화 CLI를 만드는 것이다.