# Helm Chart 설치 가이드

이 가이드는 Helm 차트를 사용하여 CUBRID Operator를 설치하는 방법을 상세히 설명합니다.

## 사전 요구 사항

- Kubernetes 1.29+
- Helm 3.0+
- kubectl 1.11.3+

## 차트 구성

CUBRID Operator의 Helm 차트는 두 개의 차트로 구성되어 있습니다:
- `cubrid-operator-crds`: CRD(Custom Resource Definitions)
- `cubrid-operator`: Operator 컨트롤러 및 관련 리소스

## 설치 방법

CUBRID Operator를 설치하는 방법에는 두 가지가 있습니다:
1. Helm Repository를 사용하는 방법 (권장)
2. 로컬 Helm Chart Package를 사용하는 방법

### 방법 1: Helm Repository 사용

#### 1. Helm Repository 추가

```bash
# Helm repo 추가
helm repo add cubrid https://cubrid.github.io/helm-charts
helm repo update
```

#### 2. CRD 설치

```bash
# CRD 차트 설치
helm install cubrid-operator-crds cubrid/cubrid-operator-crds

# CRD 설치 확인
kubectl get crds | grep cubrid
```

#### 3. Operator 설치

```bash
# 기본 설정으로 설치 (default namespace)
helm install cubrid-operator cubrid/cubrid-operator

# 특정 namespace에 설치 (namespace가 없으면 자동 생성)
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid --create-namespace

# 특정 namespace에 설치 (기존 namespace 사용)
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid-prod

# 특정 버전 설치
helm install cubrid-operator cubrid/cubrid-operator --version <version> --namespace cubrid --create-namespace
```

### 방법 2: 로컬 Helm Chart Package 사용

#### 1. Chart Package 생성성

```bash
# 소스 디렉토리에서
cd deploy/charts

# Chart Package 생성
helm package cubrid-operator-crds
helm package cubrid-operator
```

#### 2. CRD 설치

```bash
# CRD Chart Package 설치
helm install cubrid-operator-crds ./cubrid-operator-crds-<version>.tgz

# CRD 설치 확인
kubectl get crds | grep cubrid
```

#### 3. Operator 설치

```bash
# 기본 설정으로 설치 (default namespace)
helm install cubrid-operator ./cubrid-operator-<version>.tgz

# 특정 namespace에 설치 (namespace가 없으면 자동 생성)
helm install cubrid-operator ./cubrid-operator-<version>.tgz --namespace cubrid --create-namespace

# 특정 namespace에 설치 (기존 namespace 사용)
helm install cubrid-operator ./cubrid-operator-<version>.tgz --namespace cubrid-prod

# values.yaml 파일 사용
helm install cubrid-operator ./cubrid-operator -f values.yaml --namespace cubrid --create-namespace
```

## 사용자 정의 설정

CUBRID Operator의 설치 시 다양한 설정 옵션을 통해 환경에 맞게 커스터마이징할 수 있습니다. 주요 설정 영역은 RBAC 권한 관리와 Webhook 인증서 관리입니다.

### Namespace 관리

CUBRID Operator는 다양한 namespace 관리 방법을 지원합니다:

#### 방법 1: --create-namespace 옵션 사용 (권장)
```bash
# namespace가 없으면 자동으로 생성
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid --create-namespace
```

#### 방법 2: 수동으로 namespace 생성
```bash
# 1. namespace 수동 생성
kubectl create namespace cubrid

# 2. Helm chart 설치
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid
```

#### 방법 3: Chart에서 자동 생성
```bash
# values.yaml에서 createNamespace: true 설정 후 설치
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid --set createNamespace=true
```

### RBAC 설정

```bash
# User/Editor 권한 포함하여 설치
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set rbac.createUserRoles=true \
  --set rbac.createEditorRoles=true

# Viewer 권한 비활성화하여 설치
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set rbac.createViewerRoles=false
```

### Webhook 설정

CUBRID Operator의 webhook 서버는 두 가지 방식으로 TLS 인증서를 관리할 수 있습니다:

```bash
# 외부 cert-manager 사용
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set webhook.certManagerType=external

# 내부 cert-manager 사용 (기본값)
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set webhook.certManagerType=internal
```

자세한 인증서 관리 설정 방법은 [cert-manager-guide.md](./cert-manager-guide.md)를 참조하세요.

## 업그레이드

### Repository 사용 시

```bash
# Helm repository 업데이트
helm repo update

# CRD 업그레이드
helm upgrade cubrid-operator-crds cubrid/cubrid-operator-crds

# Operator 업그레이드
helm upgrade cubrid-operator cubrid/cubrid-operator
```

### 로컬 Package 사용 시

```bash
# 새 버전의 Chart Package로 업그레이드
helm upgrade cubrid-operator-crds ./cubrid-operator-crds-<new-version>.tgz
helm upgrade cubrid-operator ./cubrid-operator-<new-version>.tgz
```

## 제거

```bash
# Operator 제거 (특정 namespace에서)
helm uninstall cubrid-operator --namespace cubrid

# CRD 제거
helm uninstall cubrid-operator-crds

# namespace도 함께 제거하려면
kubectl delete namespace cubrid
```

## Helm 설정 옵션

| 매개변수 | 설명 | 기본값 |
|----------|------|---------|
| `namespace` | Operator가 설치될 namespace | `""` (Release.Namespace 사용) |
| `createNamespace` | Chart에서 namespace 자동 생성 여부 | `false` |
| `rbac.createUserRoles` | User 권한 생성 여부 | `false` |
| `rbac.createEditorRoles` | Editor 권한 생성 여부 | `false` |
| `rbac.createViewerRoles` | Viewer 권한 생성 여부 | `true` |
| `webhook.certManagerType` | 인증서 관리자 타입 (internal/external) | `internal` |

더 자세한 설정 옵션은 [values.yaml](../deploy/charts/cubrid-operator/values.yaml) 또는 [Helm Chart 매개변수](./helm-chart-parameter.md)를 참조하세요.
