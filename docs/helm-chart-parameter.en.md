# Helm Chart Parameters

## Overview

Detailed descriptions of all configuration parameters for the CUBRID Operator Helm Chart are provided. These parameters can be customized during Helm installation using the `--set` flag or `values.yaml` file.

## Parameters

### Common Parameters

| Name | Description | Value |
|------|-------------|-------|
| `nameOverride` | String to partially override common.names.fullname | `""` |
| `fullnameOverride` | String to completely override common.names.fullname | `""` |
| `namespace` | Operator namespace | `""` (uses Release.Namespace) |
| `createNamespace` | Whether to auto-create namespace from Chart | `false` |
| `currentNamespaceOnly` | Whether operator watches only CRDs in its own namespace | `false` |

### Operator Parameters

| Name | Description | Value |
|------|-------------|-------|
| `operator.enabled` | Enable operator deployment | `true` |
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

### Webhook Parameters

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

### Image Parameters

| Name | Description | Value |
|------|-------------|-------|
| `image.repository` | Operator image repository | `"cubrid/operator"` |
| `image.tag` | Operator image tag | `"latest"` |
| `image.pullPolicy` | Image pull policy | `"IfNotPresent"` |
| `imagePullSecrets` | Image pull secrets | `[]` |

### Resource Parameters

| Name | Description | Value |
|------|-------------|-------|
| `webhook.resources.limits.cpu` | CPU limit | `"100m"` |
| `webhook.resources.limits.memory` | Memory limit | `"512Mi"` |
| `webhook.resources.requests.cpu` | CPU request | `"50m"` |
| `webhook.resources.requests.memory` | Memory request | `"256Mi"` |

### Probe Parameters

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

### Ingress Parameters

| Name | Description | Value |
|------|-------------|-------|
| `ingress.enabled` | Enable Ingress | `true` |

## Namespace-related Considerations

### Permission Requirements
- Creating namespaces may require cluster-admin permissions
- Regular users can only deploy to existing namespaces

### Conflict Prevention
- Errors occur when trying to create in already existing namespaces
- Using the `--create-namespace` option handles this automatically

### Resource Cleanup
- Namespaces are not automatically deleted with `helm uninstall`
- To also remove the namespace, manually use the `kubectl delete namespace` command 