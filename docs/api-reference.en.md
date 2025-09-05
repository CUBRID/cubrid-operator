# CUBRID Operator API Reference

## Overview

The API fields of Custom Resource Definitions (CRDs) used in CUBRID Operator are described.

## Table of Contents

- [CubridDB](#cubriddb)
  - [CubridDBSpec](#cubriddbspec)
  - [CubridDBStatus](#cubriddbstatus)
- [BackupDB](#backupdb)
  - [BackupDBSpec](#backupdbspec)
  - [BackupDBStatus](#backupdbstatus)

---

## CubridDB

CubridDB is a Custom Resource for managing CUBRID database instances.

**API Group**: `k8s.cubrid.com`  
**Version**: `v1`  
**Kind**: `CubridDB`  
**Scope**: `Namespaced`

### CubridDBSpec

CubridDBSpec defines the desired state of CubridDB.

**ContainerTemplate Fields:**
CubridDBSpec embeds ContainerTemplate to include container-related settings.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `imagePullPolicy` | [PullPolicy](#pullpolicy) | No | `IfNotPresent` | Image pull policy |
| `resources` | [ResourceRequirements](#resourcerequirements) | No | - | Container resource requirements (CPU, memory) |
| `replication` | [Replication](#replication) | No | `{enable: false, replicas: 1}` | Replication settings |
| `affinity` | [Affinity](#affinity) | No | `{enableAntiAffinity: false}` | Pod affinity settings |
| `broker` | [Broker](#broker)[] | No | `[]` | Broker service settings |
| `cmsService` | [CMSService](#CMSService) | No | `{enabled: false}` | CMS service settings |
| `image` | string | No | `cubrid/cubrid` | CUBRID image |
| `initContainerImage` | string | No | `busybox` | Init container image |
| `storage` | [Storage](#storage)[] | No | `[]` | Storage settings |
| `label` | string | No | `""` | Additional labels |
| `updateStrategy` | [StatefulSetUpdateStrategy](#statefulsetupdatestrategy) | No | `{type: RollingUpdate}` | Update strategy |

#### Replication

Defines replication settings.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enable` | boolean | No | `false` | Whether to enable replication |
| `replicas` | int32 | No | `1` | Number of replicas |
| `haPort` | int32 | No | `59901` | HA port number |
| `hamodeType` | [HAmodeType](#hamodetype) | No | `{type: "master-slave"}` | HA mode type |

#### HAmodeType

Defines HA mode type.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | No | `"master-slave"` | HA mode type (`master-slave`, `replica`) |
| `cubridRef` | [CubridRef](#cubridref) | No | `{}` | CUBRID instance to reference |

#### CubridRef

Defines the CUBRID instance to reference.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | No | `""` | Name of CUBRID instance to reference |
| `namespace` | string | No | `"default"` | Namespace of CUBRID instance to reference |

#### Affinity

Defines pod affinity settings.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enableAntiAffinity` | boolean | No | `false` | Whether to enable anti-affinity |

#### Broker

Defines broker service settings. Multiple brokers can be configured as an array, with a default of 2.

**Default Broker Settings:**
- **First Broker**: `cubrid-query-editor` (port: 30000, NodePort: 30000)
- **Second Broker**: `cubrid-broker1` (port: 33000, NodePort: 31000)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **Yes** | - | Broker service name |
| `port` | int32 | No | `30000` | Broker port number |
| `serviceType` | [ServiceType](#servicetype) | **Yes** | `ClusterIP` | Service type (`ClusterIP`, `NodePort`) |
| `servicePort` | int32 | No | - | NodePort service port number (when serviceType is NodePort) |

#### CMSService

Defines CMS service settings.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | boolean | No | `false` | Whether to enable CMS service |
| `type` | string | No | `NodePort` | CMS service type (`NodePort`, `Ingress`) |
| `port` | int32 | No | `8001` | CMS port number |
| `startPort` | int32 | No | `31000` | Starting NodePort number |

#### Storage

Defines storage settings.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **Yes** | - | Storage name |
| `type` | string | **Yes** | - | Storage type (`database`, `backup`, `logs`) |
| `mountPath` | string | **Yes** | - | Mount path in container |
| `volumeClaimTemplate` | [VolumeClaimTemplate](#volumeclaimtemplate) | **Yes** | - | Volume claim template |

#### VolumeClaimTemplate

Defines volume claim template settings.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `metadata` | [ObjectMeta](#objectmeta) | No | - | Metadata for PVC |
| `spec` | [PersistentVolumeClaimSpec](#persistentvolumeclaimspec) | **Yes** | - | PVC specification |

### CubridDBStatus

CubridDBStatus represents the current state of CubridDB.

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | [Condition](#condition)[] | Current service state conditions |
| `replicas` | int32 | Number of replicas |
| `readyReplicas` | int32 | Number of ready replicas |
| `currentReplicas` | int32 | Number of current replicas |
| `updatedReplicas` | int32 | Number of updated replicas |

---

## BackupDB

BackupDB is a Custom Resource for managing CUBRID database backups.

**API Group**: `k8s.cubrid.com`  
**Version**: `v1`  
**Kind**: `BackupDB`  
**Scope**: `Namespaced`

### BackupDBSpec

BackupDBSpec defines the desired state of BackupDB.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `cubridRef` | [CubridRef](#cubridref) | **Yes** | - | Reference to CUBRID instance |
| `schedule` | string | No | - | Cron schedule for backup |
| `retention` | int32 | No | `7` | Number of backups to retain |
| `storage` | [Storage](#storage) | **Yes** | - | Backup storage settings |

### BackupDBStatus

BackupDBStatus represents the current state of BackupDB.

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | [Condition](#condition)[] | Current service state conditions |
| `lastBackupTime` | [Time](#time) | Time of last backup |
| `backupCount` | int32 | Number of backups created |

---

## Common Types

### PullPolicy

Defines image pull policy.

- `Always`: Always pull the image
- `IfNotPresent`: Pull the image only if it is not present
- `Never`: Never pull the image

### ServiceType

Defines service type.

- `ClusterIP`: ClusterIP service
- `NodePort`: NodePort service
- `LoadBalancer`: LoadBalancer service

### Condition

Defines condition status.

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Type of condition |
| `status` | string | Status of condition (`True`, `False`, `Unknown`) |
| `reason` | string | Reason for condition |
| `message` | string | Human-readable message |

### Time

Represents time in RFC3339 format.

### ObjectMeta

Standard Kubernetes ObjectMeta.

### PersistentVolumeClaimSpec

Standard Kubernetes PersistentVolumeClaimSpec.

### ResourceRequirements

Standard Kubernetes ResourceRequirements.

### StatefulSetUpdateStrategy

Standard Kubernetes StatefulSetUpdateStrategy.

---

## Examples

### Basic CubridDB Example

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid-db
spec:
  image: cubrid/cubrid:latest
  replication:
    enable: false
    replicas: 1
  broker:
    - name: query-editor
      port: 30000
      serviceType: NodePort
      servicePort: 30000
    - name: broker1
      port: 33000
      serviceType: NodePort
      servicePort: 31000
  storage:
    - name: database
      type: database
      mountPath: /opt/cubrid/databases
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 10Gi
```

### HA CubridDB Example

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: ha-cubrid-db
spec:
  image: cubrid/cubrid:latest
  replication:
    enable: true
    replicas: 3
    haPort: 59901
    hamodeType:
      type: master-slave
  affinity:
    enableAntiAffinity: true
  broker:
    - name: query-editor
      port: 30000
      serviceType: NodePort
      servicePort: 30000
  cmsService:
    enabled: true
    type: NodePort
    port: 8001
    startPort: 31000
  storage:
    - name: database
      type: database
      mountPath: /opt/cubrid/databases
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 10Gi
```

### BackupDB Example

```yaml
apiVersion: k8s.cubrid.com/v1
kind: BackupDB
metadata:
  name: my-backup
spec:
  cubridRef:
    name: my-cubrid-db
    namespace: default
  schedule: "0 2 * * *"  # Daily at 2 AM
  retention: 7
  storage:
    name: backup
    type: backup
    mountPath: /opt/cubrid/backup
    volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
``` 