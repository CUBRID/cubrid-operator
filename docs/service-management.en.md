# CUBRID Operator Service Management

## Overview

The service management of CUBRID Operator is explained.

CUBRID Operator manages various services for CUBRID database instances. All services are managed through **CR-based management** and must be configured and modified through Custom Resource (CR) specifications. Direct modification through `kubectl edit svc` is not recommended.

## Service Types

### 1. Broker Services
- **Purpose**: Expose CUBRID broker ports for client connections
- **Configuration**: Defined in `spec.broker[]` array
- **Management**: CR-based management
- **Modification**: Use `kubectl edit cubriddb <name>`

### 2. Headless Service
- **Purpose**: Provide stable network identifiers for StatefulSet pods in HA mode
- **Configuration**: Auto-generated (when HA mode is enabled)
- **Management**: CR-based management
- **Modification**: Use `kubectl edit cubriddb <name>`

### 3. CMS Service
- **Purpose**: Expose CUBRID Manager Server for management
- **Configuration**: Service type determined by `spec.cmsService.type`
- **Management**: CR-based management
- **Types**:
  - **NodePort**: `type: "NodePort"` - Creates NodePort service
  - **Ingress**: `type: "Ingress"` - Creates Ingress CMS service + Ingress resource
- **Modification**: Use `kubectl edit cubriddb <name>`
- **Limitation**: Service type cannot be changed after creation (immutable)

## Service Management Principles

### CR-based Management Services
All services are managed through Custom Resource specifications. This ensures:

1. **Consistency**: All service configurations stored in one place
2. **Version Control**: Service changes can be tracked through Git
3. **Automation**: Services are automatically adjusted to match CR state
4. **Protection**: Direct modifications to services are automatically reverted

### Automatic Cleanup
- **Owner References**: 
  - Broker, headless, and CMS services have owner references to CubridDB resources
  - Ingress CMS services have owner references to their respective Pod resources
- **Pod Lifecycle**: Ingress CMS services are automatically created for new pods and deleted when pods are removed
- **Resource Cleanup**: When CubridDB is deleted, all related services are automatically cleaned up
- **Pod Deletion**: When individual pods are deleted, related Ingress CMS services are automatically cleaned up

## Configuration Examples

### Broker Service Configuration
```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid
spec:
  broker:
    - name: query-editor
      port: 33000
      servicePort: 30000
      serviceType: NodePort
    - name: broker1
      port: 33001
      servicePort: 30001
      serviceType: NodePort
```

### CMS Service Configuration
```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid
spec:
  cmsService:
    type: nodePort
    port: 8001
    startPort: 31000
```

### HA Port Configuration
```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid
spec:
  replication:
    enable: true
    replicas: 2
    haPort: 59901  # Port used only in HA mode
    hamodeType:
      type: master-slave
```

## Service Modification
Service modifications are only possible through CR. Direct service modifications will revert to previous settings.

### Modification through CR
```bash
kubectl edit cubriddb my-cubrid
```

### Simple Configuration Examples

#### Change Broker Service Port
```bash
# 1. Edit CR
kubectl edit cubriddb my-cubrid

# 2. Modify port in spec.broker section
spec:
  broker:
    - name: query-editor
      port: 33000
      servicePort: 30001  # Change from 30000 to 30001
      serviceType: NodePort
```

#### Change CMS Service Type (Limited)
```bash
# CMS Service type cannot be changed after creation
# Attempting to change NodePort → Ingress or Ingress → NodePort will cause errors

# Wrong example (will cause error)
spec:
  cmsService:
    type: Ingress  # Attempting to change from existing NodePort → Error

# Correct method: Create new CubridDB instance
# 1. Backup existing instance
# 2. Create new instance (with desired type)
# 3. Data migration
```

#### Change HA Port
```bash
# 1. Edit CR
kubectl edit cubriddb my-cubrid

# 2. Modify port in spec.replication section
spec:
  replication:
    enable: true
    replicas: 2
    haPort: 59902  # Change from 59901 to 59902
```

#### Change Replicas Count (Automatic Service Count Adjustment)
```bash
# 1. Edit CR
kubectl edit cubriddb my-cubrid

# 2. Modify replicas in spec.replication section
spec:
  replication:
    enable: true
    replicas: 3  # Change from 2 to 3 (CMS services also automatically adjusted to 3)
```

## Service Naming Rules

### Broker Services
- **Format**: `{broker.name}`
- **Examples**: `query-editor`, `broker1`

### CMS NodePort Services
- **Format**: `{cubriddb-name}-{namespace}-cms-{pod-index}`
- **Examples**: `my-cubrid-default-cms-0`, `my-cubrid-default-cms-1`

### Ingress CMS Services
- **Format**: `{pod-name}-{namespace}-cms-svc`
- **Examples**: `my-cubrid-0-default-cms-svc`, `my-cubrid-1-default-cms-svc`

### Headless Services
- **Format**: `{cubriddb-name}-headless`
- **Examples**: `my-cubrid-headless`

## Detailed Information by Service Type

### Broker Services
- **Service Type**: NodePort (currently supported)
- **Port Configuration**: Broker port → NodePort mapping
- **Auto Generation**: Based on `spec.broker[]` array in CR
- **Owner**: CubridDB resource

### CMS NodePort Services
- **Service Type**: NodePort
- **Port Configuration**: CMS port → NodePort mapping
- **Auto Generation**: Created per pod
- **Owner**: Respective pod
- **Condition**: Created when `spec.cmsService.type` is NodePort

### Ingress CMS Services
- **Service Type**: ClusterIP (Headless)
- **Port Configuration**: Uses CMS port
- **Auto Generation**: Created per pod
- **Owner**: Respective pod
- **Condition**: Created when `spec.cmsService.type` is Ingress

### Headless Services
- **Service Type**: ClusterIP (Headless)
- **Port Configuration**: Uses HA port
- **Auto Generation**: When HA mode is enabled
- **Owner**: CubridDB resource
- **Condition**: Only created when `spec.replication.enable: true` 