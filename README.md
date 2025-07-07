# CUBRID Operator

CUBRID Operator는 Kubernetes 환경에서 CUBRID 데이터베이스 클러스터를 관리하기 위한 Kubernetes Operator입니다. 이 Operator는 CUBRID 데이터베이스의 자동화된 배포, 스케일링, 백업, 고가용성 관리를 제공합니다.

## Description

CUBRID Operator는 Kubernetes의 Custom Resource Definitions (CRDs)를 사용하여 CUBRID 데이터베이스 인스턴스를 관리합니다. 이 Operator는 다음과 같은 목적으로 설계되었습니다:

- **자동화된 배포**: CUBRID 데이터베이스 인스턴스의 자동 배포 및 구성
- **고가용성**: Master-Slave 복제를 통한 고가용성 구성
- **백업 관리**: 자동화된 백업 스케줄링 및 관리
- **스케일링**: 수평 및 수직 스케일링 지원
- **모니터링**: 상태 모니터링 및 메트릭 수집

## Features

### Core Features
- **CubridDB CRD**: CUBRID 데이터베이스 인스턴스 관리
  - 단일 인스턴스 배포
  - Master-Slave 복제 구성
  - 샤딩 지원
  - 브로커 서비스 관리
  - 롤링 업데이트

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
- **메트릭 수집**: Prometheus 메트릭 지원
- **Helm Chart**: 쉬운 배포 및 관리
- **다중 아키텍처 지원**: AMD64, ARM64 등

## Getting Started

### Prerequisites

- **Kubernetes**: 1.29+
- **Helm**: 3.0+
- **Go**: 1.21+ (개발용)
- **Docker**: 17.03+ (이미지 빌드용)
- **kubectl**: 1.11.3+
- **PV provisioner**: 기본 인프라에서 지원

### Quick Start

#### Install Operator

##### Helm Repository 방식

1. **Helm repository 추가**:
```bash
helm repo add cubrid-operator https://<your-helm-repo-url>
helm repo update
```

2. **CRDs 설치**:
```bash
helm install cubrid-operator-crds cubrid-operator/cubrid-operator-crds
```

3. **Operator 설치**:
```bash
helm install cubrid-operator cubrid-operator/cubrid-operator
```

##### Helm Chart (로컬 디렉토리) 방식

1. **CRDs 설치**:
```bash
helm install cubrid-operator-crds ./config/charts/cubrid-operator-crds
```

2. **Operator 설치**:
```bash
# 기본 설치 (모든 ClusterRole 포함)
helm install cubrid-operator ./config/charts/cubrid-operator

# RBAC ClusterRole 없이 설치 (수동 권한 관리)
helm install cubrid-operator ./config/charts/cubrid-operator \
  --set rbac.createUserRoles=false

# Editor 권한만 배포 (Viewer 권한은 수동 관리)
helm install cubrid-operator ./config/charts/cubrid-operator \
  --set rbac.createViewerRoles=false
```

##### YAML 방식

1. **모든 리소스 설치**:
```bash
kubectl apply -f dist/cubrid-operator-install.yaml
```

##### Make deploy 방식

1. **이미지 빌드 및 푸시**:
```bash
make docker-build docker-push IMG=<registry>/cubrid-operator:tag
```

2. **CRDs 설치**:
```bash
make install-crds
```

3. **Operator 배포**:
```bash
make deploy IMG=<registry>/cubrid-operator:tag
```

#### Upgrade Operator

##### Helm Repository 방식

```bash
# Helm repository 업데이트
helm repo update

# Operator 업그레이드
helm upgrade cubrid-operator cubrid-operator/cubrid-operator

# 특정 버전으로 업그레이드
helm upgrade cubrid-operator cubrid-operator/cubrid-operator --version <version>
```

##### Helm Chart (로컬 디렉토리) 방식

```bash
# Operator 업그레이드
helm upgrade cubrid-operator ./config/charts/cubrid-operator

# 새로운 설정값과 함께 업그레이드
helm upgrade cubrid-operator ./config/charts/cubrid-operator \
  --set rbac.createUserRoles=false \
  --set replicaCount=2

# 업그레이드 전 롤백 가능하도록 히스토리 유지
helm upgrade cubrid-operator ./config/charts/cubrid-operator \
  --history-max=10
```

##### 업그레이드 상태 확인

```bash
# 업그레이드 상태 확인
helm status cubrid-operator

# 업그레이드 히스토리 확인
helm history cubrid-operator

# 롤백 (필요시)
helm rollback cubrid-operator <revision>
```

#### Uninstall Operator

##### Helm Chart 방식

```bash
# Operator 제거
helm uninstall cubrid-operator

# CRDs 제거
helm uninstall cubrid-operator-crds
```

##### YAML 방식

```bash
kubectl delete -f dist/cubrid-operator-install.yaml
```

##### Make undeploy 방식

```bash
# Operator 제거
make undeploy

# CRDs 제거
make uninstall-crds
```

### Create a CubridDB, BackupDB CR

#### CubridDB CR 설치

**단일 인스턴스 CUBRID 데이터베이스 생성**:

```bash
kubectl apply -f - <<EOF
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid-db
spec:
  image: airnet73/cubrid
  initContainerImage: busybox
  imagePullPolicy: Always
  replication:
    enable: false
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
  image: airnet73/cubrid
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

```bash
# 모든 CubridDB 리소스 조회
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

CUBRID Operator의 CRD API 필드에 대한 상세한 설명은 [API Reference](./docs/API_REFERENCE.md)를 참조하세요.

### Storage Provisioning

CUBRID Operator는 동적 프로비저닝과 정적 프로비저닝을 모두 지원합니다. 상세한 설정 방법과 예시는 [스토리지 프로비저닝 가이드](./docs/STORAGE_PROVISIONING.md)를 참조하세요.

### Service Management

CUBRID Operator는 브로커 서비스, CMS 서비스, 헤드리스 서비스 등을 CR 기반으로 관리합니다. 서비스 구성 및 관리 방법은 [서비스 관리 가이드](./docs/SERVICE_MANAGEMENT.md)를 참조하세요.

### Helm Chart Parameters

자세한 Helm Chart 파라미터는 [Helm Chart README](./config/charts/cubrid-operator/README.md)를 참조하세요.

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

```bash
make build-installer IMG=<registry>/cubrid-operator:tag
```

이 명령은 `dist/` 디렉토리에 `cubrid-operator-install.yaml` 파일을 생성합니다.

### 배포

사용자는 다음 명령으로 프로젝트를 설치할 수 있습니다:

```bash
kubectl apply -f https://raw.githubusercontent.com/<org>/cubrid-operator/<tag>/dist/cubrid-operator-install.yaml
```

## Contributing

프로젝트에 기여하고 싶으시다면 [Contributing Guide](./CONTRIBUTING.md)를 참조하세요.

### Development

```bash
# 의존성 설치
make kustomize controller-gen

# 매니페스트 생성
make manifests

# 코드 생성
make generate

# 빌드
make build

# 테스트
make test
```

**참고**: `make help`를 실행하여 사용 가능한 모든 make 타겟을 확인할 수 있습니다.


## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Support

- **GitHub Issues**: [https://github.com/CUBRID/cubrid-operator/issues](https://github.com/CUBRID/cubrid-operator/issues)
- **Documentation**: [https://github.com/CUBRID/cubrid-operator](https://github.com/CUBRID/cubrid-operator)
- **CUBRID Documentation**: [https://www.cubrid.org/documentation](https://www.cubrid.org/documentation)



