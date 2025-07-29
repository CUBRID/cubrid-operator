# CUBRID Operator Storage Provisioning Guide

CUBRID Operator supports two storage provisioning methods: **Dynamic Provisioning** and **Static Provisioning**.

## Table of Contents

- [Overview](#overview)
- [Dynamic Provisioning](#dynamic-provisioning)
- [Static Provisioning](#static-provisioning)
- [Choosing Provisioning Method](#choosing-provisioning-method)
- [Example Files](#example-files)
- [Troubleshooting](#troubleshooting)

## Overview

The storage provisioning methods of CUBRID Operator are explained.

### Dynamic Provisioning
- Automatically creates PersistentVolume(PV) using StorageClass
- PV is automatically provisioned when PVC is created
- Recommended method for cloud environments or storage systems

### Static Provisioning
- Administrator creates and manages PV in advance
- PVC is bound to existing PV
- Used for on-premises environments or specific storage requirements

## Dynamic Provisioning

### Characteristics
- **Automation**: Automatically creates PV when PVC is created
- **Scalability**: Automatically expands storage as needed
- **Management Convenience**: No manual PV management required

### Configuration Method

#### 1. Check StorageClass
First, check available StorageClasses in the cluster:

```bash
kubectl get storageclass
```

Common StorageClass examples:
- `longhorn` (Longhorn storage)
- `standard` (Default storage)
- `fast-ssd` (High-performance SSD)

#### 2. CubridDB Configuration

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: cubrid-dynamic
spec:
  replication:
    enable: false
    replicas: 1
  storage:
    - name: database
      type: database
      mountPath: /opt/cubrid/databases
      volumeClaimTemplate:
        metadata:
          name: database
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 10Gi
          storageClassName: longhorn  # StorageClass for dynamic provisioning
  broker:
    - name: broker-svc
      port: 30000
      serviceType: ClusterIP
```

#### 3. Apply and Verify

```bash
# Create CubridDB
kubectl apply -f cubrid-dynamic.yaml

# Check PVC status
kubectl get pvc

# Check PV status
kubectl get pv

# Check pod status
kubectl get pods -l app=cubriddb
```

### Dynamic Provisioning Example File

Refer to the `config/samples/cubrid-dynamic-provisioning.yaml` file.

## Static Provisioning

### Characteristics
- **Manual Management**: Administrator creates PV in advance
- **Controllable**: Precise control over storage location and characteristics
- **Predictable Performance**: Guaranteed predefined storage performance

### Configuration Method

#### 1. Create PersistentVolume

First, create the PV to be used:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: cubrid-database-pv-0
  labels:
    type: cubrid-database
    app: cubrid-database
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""  # Empty string = static provisioning
  hostPath:
    path: /data/cubrid/database-0
    type: DirectoryOrCreate
```

#### 2. Create Directory and Set Permissions

Create directory on nodes and set permissions:

```bash
# Run on each node
sudo mkdir -p /data/cubrid/database-0
sudo chown 1000:1000 /data/cubrid/database-0
sudo chmod 755 /data/cubrid/database-0
```

#### 3. CubridDB Configuration

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: cubrid-static
spec:
  replication:
    enable: false
    replicas: 1
  storage:
    - name: database
      type: database
      mountPath: /home/cubrid/CUBRID/databases
      volumeClaimTemplate:
        metadata:
          name: database
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 10Gi
          storageClassName: ""  # Empty string = static provisioning
          selector:
            matchLabels:
              type: cubrid-database
              app: cubrid-static
  broker:
    - name: broker-svc
      port: 30000
      serviceType: ClusterIP
```

#### 4. Application Order

```bash
# 1. Create PV
kubectl apply -f cubrid-pv.yaml

# 2. Check PV status
kubectl get pv

# 3. Create CubridDB
kubectl apply -f cubrid-static.yaml

# 4. Check PVC status
kubectl get pvc

# 5. Check pod status
kubectl get pods -l app=cubrid-static
```

### Static Provisioning Example Files

- `config/samples/cubrid-static-provisioning.yaml`: CubridDB configuration
- `config/samples/cubrid-static-pv.yaml`: PV configuration

## Choosing Provisioning Method

### Choose Dynamic Provisioning when:
- Cloud environment (AWS, GCP, Azure)
- Storage system supports automatic provisioning
- Development/test environment
- Quick deployment is needed

### Choose Static Provisioning when:
- On-premises environment
- Specific storage location or performance requirements
- Precise control needed in production environment
- Utilizing existing storage infrastructure

## Example Files

### Dynamic Provisioning Example

```yaml
# config/samples/cubrid-dynamic-provisioning.yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: cubrid-dynamic-example
spec:
  image: airnet73/cubrid
  initContainerImage: busybox
  replication:
    enable: false
    replicas: 1
  storage:
    - name: database
      type: database
      mountPath: /opt/cubrid/databases
      volumeClaimTemplate:
        metadata:
          name: database
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 10Gi
          storageClassName: longhorn
  broker:
    - name: broker-svc
      port: 30000
      serviceType: ClusterIP
```

### Static Provisioning Example

```yaml
# config/samples/cubrid-static-provisioning.yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: cubrid-static-example
spec:
  image: airnet73/cubrid
  initContainerImage: busybox
  replication:
    enable: false
    replicas: 1
  storage:
    - name: database
      type: database
      mountPath: /opt/cubrid/databases
      volumeClaimTemplate:
        metadata:
          name: database
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 10Gi
          storageClassName: ""
          selector:
            matchLabels:
              type: cubrid-database
              app: cubrid-static-example
  broker:
    - name: broker-svc
      port: 30000
      serviceType: ClusterIP
```

```yaml
# config/samples/cubrid-static-pv.yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: cubrid-database-pv-0
  labels:
    type: cubrid-database
    app: cubrid-static-example
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""
  hostPath:
    path: /data/cubrid/database-0
    type: DirectoryOrCreate
```

## Additional Resources

- [Kubernetes Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [Storage Classes](https://kubernetes.io/docs/concepts/storage/storage-classes/)
- [API Reference](./api-reference.en.md)
- [Example Files](../config/samples/) 