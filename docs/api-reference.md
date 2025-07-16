# CUBRID Operator API Reference

이 문서는 CUBRID Operator에서 사용하는 Custom Resource Definitions (CRDs)의 API 필드들을 설명합니다.

## Table of Contents

- [CubridDB](#cubriddb)
  - [CubridDBSpec](#cubriddbspec)
  - [CubridDBStatus](#cubriddbstatus)
  - [Storage](#storage)
  - [VolumeClaimTemplate](#volumeclaimtemplate)
- [BackupDB](#backupdb)
  - [BackupDBSpec](#backupdbspec)
  - [BackupDBStatus](#backupdbstatus)

---

## CubridDB

CubridDB는 CUBRID 데이터베이스 인스턴스를 관리하기 위한 Custom Resource입니다.

**API Group**: `k8s.cubrid.com`  
**Version**: `v1`  
**Kind**: `CubridDB`  
**Scope**: `Namespaced`

### CubridDBSpec

CubridDBSpec은 CubridDB의 원하는 상태를 정의합니다.

**ContainerTemplate 필드들:**
CubridDBSpec은 ContainerTemplate을 임베드하여 컨테이너 관련 설정을 포함합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `imagePullPolicy` | [PullPolicy](#pullpolicy) | No | `IfNotPresent` | 이미지 풀 정책 |
| `resources` | [ResourceRequirements](#resourcerequirements) | No | - | 컨테이너 리소스 요구사항 (CPU, 메모리) |
| `replication` | [Replication](#replication) | No | `{enable: false, replicas: 1}` | 복제 설정 |
| `affinity` | [Affinity](#affinity) | No | `{enableAntiAffinity: false}` | 파드 어피니티 설정 |
| `broker` | [Broker](#broker)[] | No | `[]` | 브로커 서비스 설정 |
| `cmsService` | [CMSService](#CMSService) | No | `{enabled: false}` | CMS 서비스 설정 |
| `image` | string | No | `cubrid/cubrid` | CUBRID 이미지 |
| `initContainerImage` | string | No | `busybox` | 초기화 컨테이너 이미지 |
| `storage` | [Storage](#storage)[] | No | `[]` | 스토리지 설정 |
| `label` | string | No | `""` | 추가 라벨 |
| `updateStrategy` | [StatefulSetUpdateStrategy](#statefulsetupdatestrategy) | No | `{type: RollingUpdate}` | 업데이트 전략 |

#### Replication

복제 설정을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enable` | boolean | No | `false` | 복제 활성화 여부 |
| `replicas` | int32 | No | `1` | 복제본 수 |
| `haPort` | int32 | No | `59901` | HA 포트 번호 |
| `hamodeType` | [HAmodeType](#hamodetype) | No | `{type: "master-slave"}` | HA 모드 타입 |

#### HAmodeType

HA 모드 타입을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | No | `"master-slave"` | HA 모드 타입 (`master-slave`, `replica`) |
| `cubridRef` | [CubridRef](#cubridref) | No | `{}` | 참조할 CUBRID 인스턴스 |

#### CubridRef

참조할 CUBRID 인스턴스를 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | No | `""` | 참조할 CUBRID 인스턴스 이름 |
| `namespace` | string | No | `"default"` | 참조할 CUBRID 인스턴스 네임스페이스 |

#### Affinity

파드 어피니티 설정을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enableAntiAffinity` | boolean | No | `false` | 안티어피니티 활성화 여부 |

#### Broker

브로커 서비스 설정을 정의합니다. 배열로 여러 개의 브로커를 설정할 수 있으며, 기본값은 2개입니다.

**기본 브로커 설정:**
- **첫 번째 브로커**: `cubrid-query-editor` (포트: 30000, NodePort: 30000)
- **두 번째 브로커**: `cubrid-broker1` (포트: 33000, NodePort: 31000)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **Yes** | - | 브로커 서비스 이름 |
| `port` | int32 | No | `30000` | 브로커 포트 번호 |
| `serviceType` | [ServiceType](#servicetype) | **Yes** | `ClusterIP` | 서비스 타입 (`ClusterIP`, `NodePort`) |
| `servicePort` | int32 | No | - | NodePort 서비스 포트 번호 (serviceType이 NodePort인 경우) |

#### CMSService

CMS 서비스 설정을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | No | `"NodePort"` | CMS 서비스 타입 (`NodePort`, `Ingress`) |
| `startPort` | int32 | No | `31000` | CMS 서비스 시작 포트 (type=NodePort인 경우) |
| `port` | int32 | No | `8001` | CMS 포트 |

**서비스 타입 설명:**
- **NodePort**: NodePort 서비스를 생성하여 CMS에 직접 접근
- **Ingress**: Ingress CMS 서비스를 생성하여 도메인 기반 접근 (Ingress Controller 필요)


#### Storage

스토리지 설정을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | No | - | 스토리지 이름 |
| `mountPath` | string | No | - | 마운트 경로 |
| `type` | string | No | - | 스토리지 타입 |
| `size` | [Quantity](#quantity) | No | - | 스토리지 크기 |
| `storageClassName` | string | No | - | 스토리지 클래스 이름 |
| `volumeName` | string | No | - | 볼륨 이름 |
| `volumeClaimTemplate` | [VolumeClaimTemplate](#volumeclaimtemplate) | No | - | PVC 템플릿 설정 |

#### VolumeClaimTemplate

PersistentVolumeClaim 템플릿을 정의합니다. 동적 프로비저닝과 정적 프로비저닝을 모두 지원합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `metadata` | [ObjectMeta](#objectmeta) | No | - | PVC 메타데이터 |
| `spec` | [PersistentVolumeClaimSpec](#persistentvolumeclaimspec) | **Yes** | - | PVC 스펙 |

**프로비저닝 타입 구분:**
- **동적 프로비저닝**: `storageClassName`이 설정된 경우, StorageClass를 통해 동적으로 PV를 생성
- **정적 프로비저닝**: `storageClassName`이 비어있고 `selector`가 설정된 경우, 미리 생성된 PV에 바인딩

#### ObjectMeta

메타데이터를 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `labels` | map[string]string | No | - | 라벨 |
| `annotations` | map[string]string | No | - | 어노테이션 |

#### PersistentVolumeClaimSpec

PersistentVolumeClaim 스펙을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `accessModes` | [PersistentVolumeAccessMode](#persistentvolumeaccessmode)[] | **Yes** | - | 접근 모드 |
| `resources` | [ResourceRequirements](#resourcerequirements) | **Yes** | - | 리소스 요구사항 |
| `storageClassName` | string | No | - | 스토리지 클래스 이름 |
| `selector` | [LabelSelector](#labelselector) | No | - | 라벨 셀렉터 (정적 프로비저닝용) |

#### PersistentVolumeAccessMode

PersistentVolume 접근 모드를 정의합니다.

| Value | Description |
|-------|-------------|
| `ReadWriteOnce` | 단일 노드에서 읽기/쓰기 |
| `ReadOnlyMany` | 여러 노드에서 읽기 전용 |
| `ReadWriteMany` | 여러 노드에서 읽기/쓰기 |

#### ResourceRequirements

리소스 요구사항을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `requests` | map[string][Quantity](#quantity) | **Yes** | - | 요청 리소스 |
| `limits` | map[string][Quantity](#quantity) | No | - | 제한 리소스 |

#### LabelSelector

라벨 셀렉터를 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `matchLabels` | map[string]string | No | - | 매치 라벨 |
| `matchExpressions` | [LabelSelectorRequirement](#labelselectorrequirement)[] | No | - | 매치 표현식 |

#### LabelSelectorRequirement

라벨 셀렉터 요구사항을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `key` | string | **Yes** | - | 라벨 키 |
| `operator` | string | **Yes** | - | 연산자 (`In`, `NotIn`, `Exists`, `DoesNotExist`) |
| `values` | string[] | No | - | 라벨 값들 |

### CubridDBStatus

CubridDBStatus는 CubridDB의 관찰된 상태를 정의합니다.

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | [Condition](#condition)[] | 리소스 상태 조건 |
| `hamode` | string | HA 모드 상태 |
| `currentMaster` | string | 현재 마스터 서버 |
| `nodeLists` | string[] | 노드 목록 |
| `lastUpdated` | [Time](#time) | 마지막 업데이트 시간 |

---

## BackupDB

BackupDB는 CUBRID 데이터베이스의 자동화된 백업을 관리하기 위한 Custom Resource입니다.

**API Group**: `k8s.cubrid.com`  
**Version**: `v1`  
**Kind**: `BackupDB`  
**Scope**: `Namespaced`

### BackupDBSpec

BackupDBSpec은 BackupDB의 원하는 상태를 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `cubridDBRef` | [CubridDBRef](#backupdb-cubriddbref) | No | `{}` | 백업할 CUBRID DB 참조 |
| `schedules` | [Schedules](#schedules) | No | `{schedule: "0 0 * * 0", filepath: "/share/scripts/backupdb.sh", args: "start demodb 0 5"}` | 백업 스케줄 설정 |
| `storageRef` | [StorageRef](#storageref) | No | `{}` | 백업 스토리지 참조 |

#### CubridDBRef (BackupDB)

백업할 CUBRID DB를 참조합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `namespace` | string | No | `"default"` | CUBRID DB 네임스페이스 |
| `names` | string[] | No | `[]` | 백업할 CUBRID DB 이름 목록 |
| `dbName` | string | No | - | 백업할 데이터베이스 이름 |

#### Schedules

백업 스케줄 설정을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `schedule` | string | No | `"0 0 * * 0"` | Cron 표현식 스케줄 |
| `filepath` | string | No | `"/share/scripts/backupdb.sh"` | 백업 스크립트 파일 경로 |
| `args` | string | No | `"start demodb 0 5"` | 백업 스크립트 인자 |

#### StorageRef

백업 스토리지 참조를 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `storageType` | string | No | - | 백업 스토리지 타입 |

### BackupDBStatus

BackupDBStatus는 BackupDB의 관찰된 상태를 정의합니다.

| Field | Type | Description |
|-------|------|-------------|
| `backupStatus` | map[string][PodsStatus](#podsstatus) | 각 파드별 백업 상태 |
| `overallStatus` | string | 전체 백업 상태 |

#### PodsStatus

파드별 백업 상태를 정의합니다.

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | 백업 상태 |
| `message` | string | 상태 메시지 |
| `lastUpdated` | [Time](#time) | 마지막 업데이트 시간 |

---

## Common Types

### ServiceType

서비스 타입을 정의합니다.

**Values**: `ClusterIP`, `NodePort`

### PullPolicy

이미지 풀 정책을 정의합니다.

**Values**: `Always`, `IfNotPresent`, `Never`

### StatefulSetUpdateStrategy

StatefulSet 업데이트 전략을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | No | `RollingUpdate` | 업데이트 타입 |
| `rollingUpdate` | [RollingUpdateStatefulSetStrategy](#rollingupdatestatefulsetstrategy) | No | - | 롤링 업데이트 설정 |

#### RollingUpdateStatefulSetStrategy

롤링 업데이트 전략을 정의합니다.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `partition` | int32 | No | `0` | 파티션 번호 |
| `maxUnavailable` | [IntOrString](#intorstring) | No | `1` | 최대 사용 불가 파드 수 |

### Quantity

리소스 수량을 정의합니다.

**Format**: `^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`

**Examples**: `100m`, `1Gi`, `500Mi`

### IntOrString

정수 또는 문자열을 정의합니다.

**Format**: 정수 또는 문자열

### Time

시간을 정의합니다.

**Format**: RFC3339 형식 (`2006-01-02T15:04:05Z07:00`)

### Condition

리소스 상태 조건을 정의합니다.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | **Yes** | 조건 타입 |
| `status` | string | **Yes** | 조건 상태 (`True`, `False`, `Unknown`) |
| `reason` | string | **Yes** | 조건 이유 |
| `message` | string | **Yes** | 조건 메시지 |
| `lastTransitionTime` | [Time](#time) | **Yes** | 마지막 전환 시간 |
| `observedGeneration` | int64 | No | 관찰된 생성 번호 |

---

## Examples

### Basic CubridDB

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid-db
spec:
  image: cubrid/cubrid
  replication:
    enable: false
  broker:
  - name: broker-svc
    port: 30000
    servicePort: 30100
    serviceType: NodePort
```

### HA CubridDB (Master-Slave)

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: ha-cubrid-db
spec:
  image: registry.cubrid.com/cubrid
  replication:
    enable: true
    replicas: 3
    hamodeType:
      type: master-slave
  cmsService:
    type: "NodePort"
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
```

### HA CubridDB (Replica)

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: replica-cubrid-db
spec:
  image: registry.cubrid.com/cubrid
  replication:
    enable: true
    replicas: 2
    hamodeType:
      type: replica
      cubridRef:
        name: ha-cubrid-db
        namespace: default
  cmsService:
    type: "NodePort"
    startPort: 32000
    port: 8001
  broker:
  - name: replica-broker-svc
    port: 30000
    servicePort: 30200
    serviceType: NodePort
```

### BackupDB

```yaml
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
```

---

## Notes

- 모든 필드는 선택적(optional)이며, 기본값이 제공됩니다.
- 필수 필드는 **Required** 열에 **Yes**로 표시됩니다.
- 시간 관련 필드는 RFC3339 형식을 사용합니다.
- 리소스 수량은 Kubernetes 표준 형식을 따릅니다.
- HA 모드에서 `replica` 타입을 사용할 때는 `cubridRef.name`이 필수입니다. 