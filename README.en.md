# CUBRID Operator

CUBRID Operator is an operator for managing CUBRID databases in Kubernetes.

## Key Features

- **Automated Deployment**: Automatic deployment and configuration of CUBRID database instances
- **High Availability**: High availability through Master-Slave-Replica architecture
- **Backup Management**: Automated backup scheduling and management
- **Scaling**: Support for horizontal and vertical scaling
- **Monitoring**: Status monitoring

## Quick Start

### 1. Install Operator

#### Installation using Helm (Recommended)

```bash
helm repo add cubrid https://helm.cubrid.org/cubrid-operator
helm repo update
helm install cubrid-operator-crds cubrid/cubrid-operator-crds
helm install cubrid-operator cubrid/cubrid-operator
```

#### Installation using Manifest files

```bash
kubectl apply -f https://raw.githubusercontent.com/CUBRID/cubrid-operator/develop/deploy/manifests/cubrid-operator.yaml
```

### 2. Create CUBRID Instance

```bash
kubectl apply -f - <<EOF
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
EOF
```

For detailed installation instructions, see the [Installation Documentation](./docs/installation.en.md).

## Development Environment Setup

This project downloads necessary Go tools to the `./bin` folder for a consistent development environment.

### Go tools

- kustomize (v5.3.0) - Kubernetes manifest management
- controller-gen (v0.14.0) - Kubernetes controller code generation
- envtest (release-0.17) - Test environment setup
- golangci-lint (v1.54.2) - Go code linter
- gofumpt (v0.8.0) - Go code formatter

For detailed development environment setup, see the [Developer Documentation](./docs/developer.en.md).

## CI/CD

This project provides an automated CI/CD pipeline through GitHub Actions.

- **Lint Checks**: Automatic golangci-lint and gofumpt checks on PR creation
- **Build Verification**: Build tests run on main branch

For detailed information, see the [CI/CD Documentation](./docs/ci-cd.en.md).

## Documentation

- [Installation Documentation](./docs/installation.en.md) - Detailed installation methods
- [API Reference](./docs/api-reference.en.md) - CRD API documentation
- [Developer Documentation](./docs/developer.en.md) - Development environment setup and contribution methods
- [Helm Chart Documentation](./docs/helm-chart.en.md) - Helm Chart usage
- [Storage Provisioning Documentation](./docs/storage-provisioning.en.md) - Storage configuration
- [Service Management Documentation](./docs/service-management.en.md) - Service configuration
- [CMS Access Documentation](./docs/cms-access.en.md) - CMS service access methods
- [Broker Endpoint Documentation](./docs/broker-endpoint-controller.en.md) - Broker endpoint management
- [Cert Manager Documentation](./docs/cert-manager.en.md) - Cert Manager configuration

## Project Information

- **Source Code**: [https://github.com/CUBRID/cubrid-operator](https://github.com/CUBRID/cubrid-operator)
- **Helm Charts**: [https://github.com/CUBRID/helm-charts](https://github.com/CUBRID/helm-charts)

## License

This project is distributed under the [Apache License 2.0](LICENSE). 