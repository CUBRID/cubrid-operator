# Helm Chart Installation Guide

## Overview

Detailed instructions on how to install CUBRID Operator using Helm charts are provided.

## Prerequisites

- Kubernetes 1.29+
- Helm 3.0+
- kubectl 1.11.3+

## Chart Structure

The CUBRID Operator Helm chart consists of two charts:
- `cubrid-operator-crds`: Custom Resource Definitions (CRDs)
- `cubrid-operator`: Operator controller and related resources

## Installation Methods

There are two ways to install CUBRID Operator:
1. Using Helm Repository (Recommended)
2. Using Local Helm Chart Package

### Method 1: Using Helm Repository

#### 1. Add Helm Repository

```bash
# Add Helm repo
helm repo add cubrid https://helm.cubrid.org/cubrid-operator
helm repo update
```

#### 2. Install CRDs

```bash
# Install CRD chart
helm install cubrid-operator-crds cubrid/cubrid-operator-crds

# Verify CRD installation
kubectl get crds | grep cubrid
```

#### 3. Install Operator

```bash
# Install with default settings (default namespace)
helm install cubrid-operator cubrid/cubrid-operator

# Install in specific namespace (auto-create if not exists)
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid --create-namespace

# Install in specific namespace (use existing namespace)
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid-prod

# Install specific version
helm install cubrid-operator cubrid/cubrid-operator --version <version> --namespace cubrid --create-namespace
```

### Method 2: Using Local Helm Chart Package

#### 1. Create Chart Package

```bash
# From source directory
cd deploy/charts

# Create Chart Package
helm package cubrid-operator-crds
helm package cubrid-operator
```

#### 2. Install CRDs

```bash
# Install CRD Chart Package
helm install cubrid-operator-crds ./cubrid-operator-crds-<version>.tgz

# Verify CRD installation
kubectl get crds | grep cubrid
```

#### 3. Install Operator

```bash
# Install with default settings (default namespace)
helm install cubrid-operator ./cubrid-operator-<version>.tgz

# Install in specific namespace (auto-create if not exists)
helm install cubrid-operator ./cubrid-operator-<version>.tgz --namespace cubrid --create-namespace

# Install in specific namespace (use existing namespace)
helm install cubrid-operator ./cubrid-operator-<version>.tgz --namespace cubrid-prod

# Use values.yaml file
helm install cubrid-operator ./cubrid-operator -f values.yaml --namespace cubrid --create-namespace
```

## Custom Configuration

CUBRID Operator installation can be customized for your environment through various configuration options. The main configuration areas are RBAC permission management and Webhook certificate management.

### Namespace Management

CUBRID Operator supports various namespace management methods:

#### Method 1: Using --create-namespace option (Recommended)
```bash
# Auto-create namespace if it doesn't exist
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid --create-namespace
```

#### Method 2: Manually create namespace
```bash
# 1. Manually create namespace
kubectl create namespace cubrid

# 2. Install Helm chart
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid
```

#### Method 3: Auto-create from Chart
```bash
# Set createNamespace: true in values.yaml and install
helm install cubrid-operator cubrid/cubrid-operator --namespace cubrid --set createNamespace=true
```

### RBAC Configuration

```bash
# Install with User/Editor permissions
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set rbac.createUserRoles=true \
  --set rbac.createEditorRoles=true

# Install with Viewer permissions disabled
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set rbac.createViewerRoles=false
```

### Webhook Configuration

CUBRID Operator's webhook server can manage TLS certificates in two ways:

```bash
# Use external cert-manager
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set webhook.certManagerType=external

# Use internal cert-manager (default)
helm install cubrid-operator cubrid/cubrid-operator \
  --namespace cubrid --create-namespace \
  --set webhook.certManagerType=internal
```

For detailed certificate management configuration, refer to [Cert Manager Documentation](./cert-manager.en.md).

## Upgrade

### When using Repository

```bash
# Update Helm repository
helm repo update

# Upgrade CRDs
helm upgrade cubrid-operator-crds cubrid/cubrid-operator-crds

# Upgrade Operator
helm upgrade cubrid-operator cubrid/cubrid-operator
```

### When using Local Package

```bash
# Upgrade with new version Chart Package
helm upgrade cubrid-operator-crds ./cubrid-operator-crds-<new-version>.tgz
helm upgrade cubrid-operator ./cubrid-operator-<new-version>.tgz
```

## Uninstall

```bash
# Uninstall Operator (from specific namespace)
helm uninstall cubrid-operator --namespace cubrid

# Uninstall CRDs
helm uninstall cubrid-operator-crds

# To also remove namespace
kubectl delete namespace cubrid
```

## Helm Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `namespace` | Namespace where Operator will be installed | `""` (uses Release.Namespace) |
| `createNamespace` | Whether to auto-create namespace from Chart | `false` |
| `rbac.createUserRoles` | Whether to create User permissions | `false` |
| `rbac.createEditorRoles` | Whether to create Editor permissions | `false` |
| `rbac.createViewerRoles` | Whether to create Viewer permissions | `true` |
| `webhook.certManagerType` | Certificate manager type (internal/external) | `internal` |

For more detailed configuration options, refer to [values.yaml](../deploy/charts/cubrid-operator/values.yaml) or [Helm Chart Parameters](./helm-chart-parameter.en.md). 