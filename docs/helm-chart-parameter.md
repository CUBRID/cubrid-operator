# Helm Chart 매개변수

## 개요

CUBRID Operator Helm Chart의 모든 설정 매개변수에 대한 상세한 설명을 제공합니다. Helm 설치 시 `--set` 플래그나 `values.yaml` 파일을 통해 이러한 매개변수들을 커스터마이징할 수 있습니다.

## 매개변수

### 공통 매개변수

| 이름 | 설명 | 값 |
|------|------|-----|
| `nameOverride` | common.names.fullname을 부분적으로 재정의하는 문자열 | `""` |
| `fullnameOverride` | common.names.fullname을 완전히 재정의하는 문자열 | `""` |
| `namespace` | operator의 네임스페이스 | `""` (Release.Namespace 사용) |
| `createNamespace` | Chart에서 namespace 자동 생성 여부 | `false` |
| `currentNamespaceOnly` | operator가 자체 네임스페이스의 CRD만 감시할지 여부 | `false` |

### Operator 매개변수

| 이름 | 설명 | 값 |
|------|------|-----|
| `operator.enabled` | operator 배포 활성화 | `true` |
| `operator.serviceAccount.enabled` | 서비스 계정 생성 활성화 | `true` |
| `operator.serviceAccount.name` | 서비스 계정 이름 | `""` |
| `operator.clusterRoleName.name` | 클러스터 역할 이름 | `""` |
| `operator.clusterRoleBindingName.name` | 클러스터 역할 바인딩 이름 | `""` |
| `operator.electionRoleName.name` | 리더 선출 역할 이름 | `""` |
| `operator.electionRoleBindingName.name` | 리더 선출 역할 바인딩 이름 | `""` |
| `operator.cubriddbEditClusterRoleName.name` | CUBRID DB 편집 클러스터 역할 이름 | `""` |
| `operator.cubriddbViewClusterRoleName.name` | CUBRID DB 보기 클러스터 역할 이름 | `""` |
| `operator.backupdbEditClusterRoleName.name` | 백업 DB 편집 클러스터 역할 이름 | `""` |
| `operator.backupdbViewClusterRoleName.name` | 백업 DB 보기 클러스터 역할 이름 | `""` |
| `operator.ports.probe` | 프로브 포트 | `8081` |

### Webhook 매개변수

| 이름 | 설명 | 값 |
|------|------|-----|
| `webhook.enabled` | webhook 배포 활성화 | `true` |
| `webhook.namespace` | webhook 네임스페이스 | `""` |
| `webhook.certDir` | 인증서 디렉토리 | `"/tmp/k8s-webhook-server/serving-certs"` |
| `webhook.secretName` | webhook 시크릿 이름 | `"cubrid-operator-webhook-cert"` |
| `webhook.certManagerType` | 인증서 관리자 타입 | `"internal"` |
| `webhook.podAnnotations` | Pod 어노테이션 | `{}` |
| `webhook.ports.webhook` | webhook 포트 | `8443` |
| `webhook.ports.metrics` | 메트릭 포트 | `8082` |
| `webhook.ports.probe` | 프로브 포트 | `8083` |
| `webhook.failurePolicy` | webhook 실패 정책 | `"Fail"` |
| `webhook.timeoutSeconds` | webhook 타임아웃 | `5` |

### 이미지 매개변수

| 이름 | 설명 | 값 |
|------|------|-----|
| `image.repository` | Operator 이미지 저장소 | `"cubrid/cubrid-operator"` |
| `image.tag` | Operator 이미지 태그 | `"latest"` |
| `image.pullPolicy` | 이미지 풀 정책 | `"IfNotPresent"` |
| `imagePullSecrets` | 이미지 풀 시크릿 | `[]` |

### 리소스 매개변수

| 이름 | 설명 | 값 |
|------|------|-----|
| `webhook.resources.limits.cpu` | CPU 제한 | `"100m"` |
| `webhook.resources.limits.memory` | 메모리 제한 | `"512Mi"` |
| `webhook.resources.requests.cpu` | CPU 요청 | `"50m"` |
| `webhook.resources.requests.memory` | 메모리 요청 | `"256Mi"` |

### 프로브 매개변수

| 이름 | 설명 | 값 |
|------|------|-----|
| `webhook.livenessProbe.httpGet.path` | Liveness 프로브 경로 | `"/healthz"` |
| `webhook.livenessProbe.httpGet.port` | Liveness 프로브 포트 | `8083` |
| `webhook.livenessProbe.initialDelaySeconds` | Liveness 프로브 초기 지연 | `10` |
| `webhook.livenessProbe.periodSeconds` | Liveness 프로브 주기 | `10` |
| `webhook.readinessProbe.httpGet.path` | Readiness 프로브 경로 | `"/readyz"` |
| `webhook.readinessProbe.httpGet.port` | Readiness 프로브 포트 | `8083` |
| `webhook.readinessProbe.initialDelaySeconds` | Readiness 프로브 초기 지연 | `10` |
| `webhook.readinessProbe.periodSeconds` | Readiness 프로브 주기 | `10` |

### Ingress 매개변수

| 이름 | 설명 | 값 |
|------|------|-----|
| `ingress.enabled` | Ingress 활성화 | `true` |

## Namespace 관련 주의사항

### 권한 요구사항
- namespace 생성에는 cluster-admin 권한이 필요할 수 있습니다
- 일반 사용자는 기존 namespace에만 배포 가능합니다

### 충돌 방지
- 이미 존재하는 namespace에 생성하려고 하면 에러가 발생합니다
- `--create-namespace` 옵션을 사용하면 자동으로 처리됩니다

### 리소스 정리
- `helm uninstall` 시 namespace는 자동으로 삭제되지 않습니다
- namespace도 함께 제거하려면 수동으로 `kubectl delete namespace` 명령을 사용해야 합니다

