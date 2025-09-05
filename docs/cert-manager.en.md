# CUBRID Operator Webhook Certificate Management Guide

## Overview

The certificate management methods used by the CUBRID Operator's webhook server are explained.

## Overview

CUBRID Operator's webhook server can manage TLS certificates in two ways:

1. Internal cert-manager (default)
   - Operator generates and manages certificates on its own
   - No separate cert-manager installation required

2. External cert-manager
   - Uses Kubernetes cert-manager for certificate management
   - Can utilize cert-manager features like automatic certificate renewal

## Prerequisites

When using external cert-manager, the following is required:

1. cert-manager v1.13.3 or higher must be installed on the cluster.
   ```bash
   # Install cert-manager
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.3/cert-manager.yaml
   ```

2. Verify that cert-manager is running properly:
   ```bash
   kubectl get pods -n cert-manager
   NAME                                       READY   STATUS    RESTARTS   AGE
   cert-manager-6576cfbf76-9pn96              1/1     Running   0          3d2h
   cert-manager-cainjector-6fd5bb64d4-vxmgb   1/1     Running   0          3d2h
   cert-manager-webhook-7f765545b6-6vtn5      1/1     Running   0          3d2h
   ```

## Deployment Methods

### 1. Deployment using Helm Charts

When deploying using Helm charts, you can select the certificate management method through the `certManagerType` value in `values.yaml`.

1. Using internal cert-manager (default):
   ```bash
   helm install cubrid-operator ./deploy/charts/cubrid-operator
   ```
   or
   ```bash
   helm install cubrid-operator ./deploy/charts/cubrid-operator --set webhook.certManagerType=internal
   ```

2. Using external cert-manager:
   ```bash
   helm install cubrid-operator ./deploy/charts/cubrid-operator --set webhook.certManagerType=external
   ```

### 2. Deployment using Make Commands

When using the `make deploy` command, you can select the certificate management method through the `CERT_MANAGER_TYPE` environment variable.

1. Using internal cert-manager (default):
   ```bash
   make deploy IMG=cubrid/cubrid-operator:latest
   ```
   or
   ```bash
   make deploy IMG=cubrid/cubrid-operator:latest CERT_MANAGER_TYPE=internal
   ```

2. Using external cert-manager:
   ```bash
   make deploy IMG=cubrid/cubrid-operator:latest CERT_MANAGER_TYPE=external
   ```

## Operation Method

### Internal cert-manager

- Operator directly generates certificates and private keys
- Certificate files are stored in `/tmp/k8s-webhook-server/serving-certs` directory
- Certificate renewal is handled directly by the Operator

### External cert-manager

- cert-manager generates certificates through Certificate/Issuer resources
- Generated certificates are stored in Secrets
- Secrets are mounted to the webhook server's `/tmp/k8s-webhook-server/serving-certs` directory
- cert-manager automatically handles certificate renewal (30 days before expiration)

## Certificate Configuration

Resources created when using external cert-manager:

1. Issuer (selfsigned)
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: cubrid-operator-webhook-selfsigned-issuer
     namespace: cubrid
   spec:
     selfSigned: {}
   ```

2. Certificate
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: cubrid-operator-webhook-serving-cert
     namespace: cubrid
   spec:
     dnsNames:
     - cubrid-operator-webhook-service.cubrid.svc
     - cubrid-operator-webhook-service.cubrid.svc.cluster.local
     issuerRef:
       kind: Issuer
       name: cubrid-operator-webhook-selfsigned-issuer
     secretName: cubrid-operator-webhook-cert
     duration: 8760h0m0s # 1 year
     renewBefore: 720h0m0s # 30 days
   ``` 