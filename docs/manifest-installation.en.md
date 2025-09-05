# Manifest Installation Guide

## Overview

Detailed instructions on how to install CUBRID Operator using Kubernetes manifests are provided.

## Prerequisites

- Kubernetes 1.29+
- kubectl 1.11.3+

## Installation Methods

### 1. Direct Installation

You can install by directly applying the manifest provided by GitHub:

```bash
kubectl apply -f https://raw.githubusercontent.com/CUBRID/cubrid-operator/v1.0/deploy/manifests/cubrid-operator.yaml
```

### 2. Installation with Local File

1. Download manifest file:
```bash
curl -O https://raw.githubusercontent.com/CUBRID/cubrid-operator/v1.0/deploy/manifests/cubrid-operator.yaml
```

2. Install:
```bash
kubectl apply -f cubrid-operator.yaml
```

### 3. Verify Installation

```bash
# Verify CRDs
kubectl get crd | grep cubrid

# Verify Operator pods
kubectl get pods -n cubrid

# Verify Operator services
kubectl get services -n cubrid
```

## Uninstall

```bash
# Uninstall using GitHub manifest
kubectl delete -f https://raw.githubusercontent.com/CUBRID/cubrid-operator/v1.0/deploy/manifests/cubrid-operator.yaml

# Or uninstall using local file
kubectl delete -f cubrid-operator.yaml
```

## Important Notes

1. When installing with manifests, internal cert-manager is used by default.
2. To use external cert-manager, Helm chart installation is recommended.
3. During uninstallation, all CUBRID-related resources (CRDs, Deployments, Services, etc.) are deleted together.

## Troubleshooting

If problems occur during installation, check the following:

1. Namespace status:
```bash
kubectl get namespace cubrid
```

2. Pod logs:
```bash
kubectl logs -n cubrid deployments/cubrid-operator-dep
kubectl logs -n cubrid deployments/cubrid-operator-webhook-dep
```

3. Events:
```bash
kubectl get events -n cubrid
``` 