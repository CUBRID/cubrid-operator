# CUBRID Operator

CUBRID Operator는 Kubernetes에서 CUBRID 데이터베이스를 관리하기 위한 operator입니다.

## Description

CUBRID Operator는 Kubernetes의 Custom Resource Definitions (CRDs)를 사용하여 CUBRID 데이터베이스 인스턴스를 관리합니다. 이 Operator는 다음과 같은 목적으로 설계되었습니다:

- **자동화된 배포**: CUBRID 데이터베이스 인스턴스의 자동 배포 및 구성
- **고가용성**: Master-Slave-Replica 복제를 통한 고가용성 구성
- **백업 관리**: 자동화된 백업 스케줄링 및 관리
- **스케일링**: 수평 및 수직 스케일링 지원
- **모니터링**: 상태 모니터링

## Features

### Core Features
- **CubridDB CRD**: CUBRID 데이터베이스 인스턴스 관리
  - 단일 인스턴스 배포
  - Master-Slave-Replica 복제 구성
  - 브로커 서비스 관리
  - CUBRID Admin과 연동
  - 롤링 업데이트
  - 리소스 관리 (CPU, 메모리 요청 및 제한)

- **BackupDB CRD**: 자동화된 백업 관리
  - Cron 표현식을 사용한 스케줄링
  - 전체 및 증분 백업 지원
  - 백업 스토리지 구성
  - 백업 상태 모니터링

### Advanced Features
- **Webhook 지원**: 리소스 검증 및 변형
- **RBAC 통합**: 세분화된 권한 관리
  - 선택적 ClusterRole 배포 지원
  - 조직별 맞춤 권한 관리 가능
- **Helm Chart**: 쉬운 배포 및 관리

## Install Operator

CUBRID Operator는 Helm Chart를 사용하여 설치하는 것을 권장합니다. 설치는 다음 두 단계로 진행됩니다:

### 1. Helm Repository 추가

```bash
# Helm repo 추가
helm repo add cubrid https://airnet73.github.io/helm-charts
helm repo update
```

### 2. CRD 설치

```bash
# CRD 설치
helm install cubrid-operator-crds cubrid/cubrid-operator-crds

# CRD 설치 확인
kubectl get crds | grep cubrid
```

### 3. Operator 설치

```bash
# CUBRID Operator 설치
helm install cubrid-operator cubrid/cubrid-operator
```

더 자세한 Helm 차트 설치 방법은 [Helm Chart 가이드](docs/helm-chart-guide.md)를 참조하세요.

### 대체 설치 방법

Helm Chart 외에도 다음 방법으로 설치할 수 있습니다:

#### make 명령어를 사용한 CRD 설치

```bash
# CRD 설치
make install-crd

# CRD 제거
make uninstall-crd
```

#### YAML 매니페스트로 설치

```bash
# CUBRID Operator 설치
kubectl apply -f https://raw.githubusercontent.com/airnet73/helm-charts/main/manifest/cubrid-operator.yaml
```

YAML 매니페스트를 사용한 상세 설치 방법은 [YAML 설치 가이드](docs/yaml-installation-guide.md)를 참조하세요.

### Webhook 인증서 관리

CUBRID Operator의 webhook 서버는 내부 또는 외부 cert-manager를 사용하여 TLS 인증서를 관리할 수 있습니다:

- 내부 cert-manager (기본값): Operator가 자체적으로 인증서 관리
- 외부 cert-manager: Kubernetes cert-manager를 사용하여 인증서 관리

자세한 설정 방법은 [Webhook 인증서 관리 가이드](docs/cert-manager-guide.md)를 참조하세요.

## Create a CubridDB, BackupDB CR

#### CubridDB CR 설치

**단일 인스턴스 CUBRID 데이터베이스 생성**:

```bash
kubectl apply -f - <<EOF
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid-db
spec:
  image: cubrid/cubrid
  initContainerImage: busybox
  imagePullPolicy: Always
  replication:
    enable: false
  resources:
    requests:
      memory: "2Gi"
      cpu: "1"
    limits:
      memory: "4Gi"
      cpu: "2"
  broker:
  - name: broker-svc
    port: 30000
    servicePort: 30100
    serviceType: NodePort
EOF
```

**고가용성 CUBRID 데이터베이스 생성**:

```bash
kubectl apply -f - <<EOF
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: ha-cubrid-db
spec:
  image: cubrid/cubrid
  initContainerImage: busybox
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      partition: 1
      maxUnavailable: 1
  replication:
    enable: true
    replicas: 3
    hamodeType:
      type: master-slave
  resources:
    requests:
      memory: "4Gi"
      cpu: "2"
    limits:
      memory: "8Gi"
      cpu: "4"
  cmsService:
    enabled: true
    startPort: 31000
    port: 8001
  broker:
  - name: ha-broker1-svc
    port: 30000
    servicePort: 30100
    serviceType: NodePort
  - name: ha-broker2-svc
    port: 33000
    servicePort: 30101
    serviceType: NodePort
EOF
```
##### CubridDB CR 상태 확인

# 모든 CubridDB 리소스 조회
```bash
kubectl get cubriddbs

# 특정 CubridDB 상세 정보 조회
kubectl describe cubriddb my-cubrid-db

# CubridDB 파드 상태 확인
kubectl get pods -l app=cubriddb

# CubridDB 서비스 확인
kubectl get services -l app=cubriddb
```

#### BackupDB CR 설치

```bash
kubectl apply -f - <<EOF
apiVersion: k8s.cubrid.com/v1
kind: BackupDB
metadata:
  name: my-backup
spec:
  cubridDBRef:
    names: ["my-cubrid-db-0"]
    namespace: default
    dbName: demodb
  schedules:
    schedule: "0 2 * * *"
    filepath: "/share/scripts/backupdb.sh"
    args: "start demodb 0 5"
  storageRef:
    storageType: backup-storage-type
EOF
```

##### BackupDB CR 상태 확인

```bash
# 모든 BackupDB 리소스 조회
kubectl get backupdbs

# 특정 BackupDB 상세 정보 조회
kubectl describe backupdb my-backup

# 백업 작업 상태 확인
kubectl get jobs -l app=backupdb
```

#### CubridDB CR 삭제

```bash
# 특정 CubridDB 삭제
kubectl delete cubriddb my-cubrid-db

# 모든 CubridDB 삭제
kubectl delete cubriddbs --all
```

#### BackupDB CR 삭제

```bash
# 특정 BackupDB 삭제
kubectl delete backupdb my-backup

# 모든 BackupDB 삭제
kubectl delete backupdbs --all
```

## Advanced Configuration

### API Reference

CUBRID Operator의 CRD API 필드에 대한 상세한 설명은 [API Reference](./docs/api-reference.md)를 참조하세요.

### Storage Provisioning

CUBRID Operator는 동적 프로비저닝과 정적 프로비저닝을 모두 지원합니다. 상세한 설정 방법과 예시는 [스토리지 프로비저닝 가이드](./docs/storage-provisioning.md)를 참조하세요.

### Service Management

CUBRID Operator는 브로커 서비스, CMS 서비스, 헤드리스 서비스 등을 CR 기반으로 관리합니다. 서비스 구성 및 관리 방법은 [서비스 관리 가이드](./docs/service-management.md)를 참조하세요.

### Helm Chart Parameters

자세한 Helm Chart 파라미터는 [Helm Chart README](./deploy/charts/cubrid-operator/README.md)를 참조하세요.

### Monitoring

Operator는 포트 8082에서 메트릭을 노출합니다:

```bash
# 메트릭 접근
kubectl port-forward -n cubrid deployment/cubrid-operator-webhook-dep 8082:8082
```

그 후 `http://localhost:8082/metrics`에서 메트릭을 확인할 수 있습니다.

### Troubleshooting

#### Operator 상태 확인

```bash
# Operator 파드 상태 확인
kubectl get pods -n cubrid

# Operator 로그 확인
kubectl logs -n cubrid deployment/cubrid-operator-controller-manager

# Webhook 상태 확인
kubectl get pods -n cubrid -l app=cubrid-operator-webhook-dep
kubectl logs -n cubrid deployment/cubrid-operator-webhook-dep
```

#### CRD 상태 확인

```bash
# CRD 확인
kubectl get crd | grep cubrid

# CR 리소스 확인
kubectl get cubriddbs
kubectl get backupdbs
```

## Project Distribution

### Installer 빌드

CUBRID Operator는 모든 필요한 리소스를 포함한 단일 설치 파일을 제공합니다.

```bash
# 설치 파일 빌드
make build-installer IMG=<registry>/cubrid-operator:tag

또는 외부 cert-manager를 사용하려면:

```bash
make build-installer IMG=<registry>/cubrid-operator:tag CERT_MANAGER_TYPE=external
```

이 명령은 `deploy/manifests/` 디렉토리에 `cubrid-operator-install.yaml` 파일을 생성합니다. 이 파일에는 다음이 포함됩니다:

- Custom Resource Definitions (CRDs)
- RBAC (Role, RoleBinding, ClusterRole, ClusterRoleBinding)
- Namespace
- Deployment (Controller Manager, Webhook)
- Service
- Validating/Mutating Webhook Configuration