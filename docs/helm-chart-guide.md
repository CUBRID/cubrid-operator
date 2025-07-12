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
helm repo add cubrid https://airnet73.github.io/helm-charts
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
# 기본 설정으로 설치
helm install cubrid-operator cubrid/cubrid-operator

# 특정 버전 설치
helm install cubrid-operator cubrid/cubrid-operator --version <version>
```

### 방법 2: 로컬 Helm Chart Package 사용

#### 1. Chart Package 다운로드

```bash
# Chart Package 다운로드 디렉토리 생성
mkdir -p helm-charts && cd helm-charts

# CRD와 Operator Chart Package 다운로드
curl -LO https://github.com/airnet73/helm-charts/releases/download/cubrid-operator-crds-<version>/cubrid-operator-crds-<version>.tgz
curl -LO https://github.com/airnet73/helm-charts/releases/download/cubrid-operator-<version>/cubrid-operator-<version>.tgz
```

또는 소스에서 직접 패키징:

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
# 기본 설정으로 설치
helm install cubrid-operator ./cubrid-operator-<version>.tgz

# values.yaml 파일 사용
helm install cubrid-operator ./cubrid-operator-<version>.tgz -f values.yaml
```

## 사용자 정의 설정

### RBAC 설정

```bash
# User/Editor 권한 포함하여 설치
helm install cubrid-operator cubrid/cubrid-operator \
  --set rbac.createUserRoles=true \
  --set rbac.createEditorRoles=true

# Viewer 권한 비활성화하여 설치
helm install cubrid-operator cubrid/cubrid-operator \
  --set rbac.createViewerRoles=false
```

### Webhook 설정

```bash
# 외부 cert-manager 사용
helm install cubrid-operator cubrid/cubrid-operator \
  --set webhook.certManagerType=external

# 내부 cert-manager 사용 (기본값)
helm install cubrid-operator cubrid/cubrid-operator \
  --set webhook.certManagerType=internal
```

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
# Operator 제거
helm uninstall cubrid-operator

# CRD 제거
helm uninstall cubrid-operator-crds
```

## 설정 옵션

| 매개변수 | 설명 | 기본값 |
|----------|------|---------|
| `rbac.createUserRoles` | User 권한 생성 여부 | `false` |
| `rbac.createEditorRoles` | Editor 권한 생성 여부 | `false` |
| `rbac.createViewerRoles` | Viewer 권한 생성 여부 | `true` |
| `webhook.certManagerType` | 인증서 관리자 타입 (internal/external) | `internal` |

더 자세한 설정 옵션은 [values.yaml](../deploy/charts/cubrid-operator/values.yaml)을 참조하세요. 