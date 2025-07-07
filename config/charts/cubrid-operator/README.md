# CUBRID Operator

[CUBRID Operator](https://github.com/CUBRID/cubrid-operator) is a Kubernetes operator for managing CUBRID database clusters. It provides automated deployment, scaling, backup, and high availability management for CUBRID databases in Kubernetes environments.

## Introduction

This chart bootstraps a [CUBRID Operator](https://github.com/CUBRID/cubrid-operator) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.29+
- Helm 3.0+
- Go 1.21+
- PV provisioner support in the underlying infrastructure

## Installing the Chart

To install the chart with the release name `cubrid-operator`:

```bash
helm install cubrid-operator ./cubrid-operator
```

The command deploys CUBRID Operator on the Kubernetes cluster in the default configuration. The [Parameters](#parameters) section lists the parameters that can be configured during installation.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `cubrid-operator` deployment:

```bash
helm uninstall cubrid-operator
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Parameters

### Common parameters

| Name | Description | Value |
|------|-------------|-------|
| `nameOverride` | String to partially override common.names.fullname | `""` |
| `fullnameOverride` | String to fully override common.names.fullname | `""` |
| `namespace` | Namespace for the operator | `"cubrid"` |
| `currentNamespaceOnly` | Whether the operator should watch CRDs only in its own namespace | `false` |

### Operator parameters

| Name | Description | Value |
|------|-------------|-------|
| `operator.enabled` | Enable the operator deployment | `true` |
| `operator.serviceAccount.enabled` | Enable service account creation | `true` |
| `operator.serviceAccount.name` | Service account name | `""` |
| `operator.clusterRoleName.name` | Cluster role name | `""` |
| `operator.clusterRoleBindingName.name` | Cluster role binding name | `""` |
| `operator.electionRoleName.name` | Leader election role name | `""` |
| `operator.electionRoleBindingName.name` | Leader election role binding name | `""` |
| `operator.cubriddbEditClusterRoleName.name` | CUBRID DB edit cluster role name | `""` |
| `operator.cubriddbViewClusterRoleName.name` | CUBRID DB view cluster role name | `""` |
| `operator.backupdbEditClusterRoleName.name` | Backup DB edit cluster role name | `""` |
| `operator.backupdbViewClusterRoleName.name` | Backup DB view cluster role name | `""` |
| `operator.ports.probe` | Probe port | `8081` |

### Webhook parameters

| Name | Description | Value |
|------|-------------|-------|
| `webhook.enabled` | Enable webhook deployment | `true` |
| `webhook.namespace` | Webhook namespace | `""` |
| `webhook.certDir` | Certificate directory | `"/tmp/k8s-webhook-server/serving-certs"` |
| `webhook.secretName` | Webhook secret name | `"cubrid-operator-webhook-cert"` |
| `webhook.certManagerType` | Certificate manager type | `"internal"` |
| `webhook.podAnnotations` | Pod annotations | `{}` |
| `webhook.ports.webhook` | Webhook port | `8443` |
| `webhook.ports.metrics` | Metrics port | `8082` |
| `webhook.ports.probe` | Probe port | `8083` |
| `webhook.failurePolicy` | Webhook failure policy | `"Fail"` |
| `webhook.timeoutSeconds` | Webhook timeout | `5` |

### Image parameters

| Name | Description | Value |
|------|-------------|-------|
| `image.repository` | Operator image repository | `"airnet73/operator"` |
| `image.tag` | Operator image tag | `"t100"` |
| `image.pullPolicy` | Image pull policy | `"IfNotPresent"` |
| `imagePullSecrets` | Image pull secrets | `[]` |

### Resource parameters

| Name | Description | Value |
|------|-------------|-------|
| `webhook.resources.limits.cpu` | CPU limit | `"100m"` |
| `webhook.resources.limits.memory` | Memory limit | `"128Mi"` |
| `webhook.resources.requests.cpu` | CPU request | `"50m"` |
| `webhook.resources.requests.memory` | Memory request | `"64Mi"` |

### Probe parameters

| Name | Description | Value |
|------|-------------|-------|
| `webhook.livenessProbe.httpGet.path` | Liveness probe path | `"/healthz"` |
| `webhook.livenessProbe.httpGet.port` | Liveness probe port | `8083` |
| `webhook.livenessProbe.initialDelaySeconds` | Liveness probe initial delay | `10` |
| `webhook.livenessProbe.periodSeconds` | Liveness probe period | `10` |
| `webhook.readinessProbe.httpGet.path` | Readiness probe path | `"/readyz"` |
| `webhook.readinessProbe.httpGet.port` | Readiness probe port | `8083` |
| `webhook.readinessProbe.initialDelaySeconds` | Readiness probe initial delay | `10` |
| `webhook.readinessProbe.periodSeconds` | Readiness probe period | `10` |

### Ingress parameters

| Name | Description | Value |
|------|-------------|-------|
| `ingress.enabled` | Enable ingress | `true` |

## Custom Resource Definitions (CRDs)

The CUBRID Operator installs the following CRDs:

### CubridDB

Manages CUBRID database instances with support for:
- Single instance deployment
- High Availability (HA) with master-slave replication
- Sharding configuration
- Broker services
- Rolling updates

**Example CubridDB resource:**

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: cubrid-single
spec:
  image: airnet73/cubrid
  initContainerImage: busybox
  imagePullPolicy: Always
  replication:
    enable: false
  broker:
  - name: s-broker1-svc
    port: 30000
    servicePort: 30100
    serviceType: NodePort
  - name: s-broker2-svc
    port: 33000
    servicePort: 30101
    serviceType: NodePort
```

**Example HA CubridDB resource:**

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: ha-ms
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
  - name: ms-broker1-svc
    port: 30000
    servicePort: 30100
    serviceType: NodePort
  - name: ms-broker2-svc
    port: 33000
    servicePort: 30101
    serviceType: NodePort
```

### BackupDB

Manages automated backup operations for CUBRID databases with:
- Scheduled backups using cron expressions
- Support for full and incremental backups
- Configurable backup storage
- Backup status monitoring

**Example BackupDB resource:**

```yaml
apiVersion: k8s.cubrid.com/v1
kind: BackupDB
metadata:
  name: backupdb-test
spec:
  cubridDBRef:
    names: 
      - "cubrid-master-0"
      - "cubrid-master-1"
      - "cubrid-master-2"
    namespace: default
    dbName: demodb
  schedules:
    schedule: "46 13 * * *"
    filepath: "/share/scripts/backupdb.sh"
    args: "start demodb 0 5"
  storageRef:
    storageType: backup-storage-type
```

## Usage

### Installing CRDs

First, install the CRDs:

```bash
helm install cubrid-operator-crds ./cubrid-operator-crds
```

### Installing the Operator

Then install the operator:

```bash
helm install cubrid-operator ./cubrid-operator
```

### Creating a CUBRID Database

Create a CUBRID database instance:

```bash
kubectl apply -f - <<EOF
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid-db
spec:
  image: airnet73/cubrid
  replication:
    enable: false
  broker:
  - name: broker-svc
    port: 30000
    servicePort: 30100
    serviceType: NodePort
EOF
```

### Creating a Backup Schedule

Create a backup schedule for your database:

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

## Monitoring

The operator exposes metrics on port 8082. You can access them via:

```bash
kubectl port-forward -n cubrid deployment/cubrid-operator-webhook-dep 8082:8082
```

Then visit `http://localhost:8082/metrics` to see the metrics.

## Troubleshooting

### Check Operator Status

```bash
kubectl get pods -n cubrid
kubectl logs -n cubrid deployment/cubrid-operator-controller-manager
```

### Check Webhook Status

```bash
kubectl get pods -n cubrid -l app=cubrid-operator-webhook-dep
kubectl logs -n cubrid deployment/cubrid-operator-webhook-dep
```

### Check CRD Status

```bash
kubectl get crd | grep cubrid
kubectl get cubriddbs
kubectl get backupdbs
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](https://github.com/CUBRID/cubrid-operator/blob/main/CONTRIBUTING.md) for details.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](https://github.com/CUBRID/cubrid-operator/blob/main/LICENSE) file for details.

## Support

- GitHub Issues: [https://github.com/CUBRID/cubrid-operator/issues](https://github.com/CUBRID/cubrid-operator/issues)
- Documentation: [https://github.com/CUBRID/cubrid-operator](https://github.com/CUBRID/cubrid-operator)
- CUBRID Documentation: [https://www.cubrid.org/documentation](https://www.cubrid.org/documentation) 