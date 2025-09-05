# BrokerEndpointReconciler

## Overview

The BrokerEndpointReconciler of CUBRID Operator is explained.

`BrokerEndpointReconciler` is one of the core components of CUBRID Operator, a controller that monitors the status of CUBRID database broker services and dynamically manages Kubernetes Endpoint resources.

## Purpose of Broker Endpoint Creation

The main reason for creating Broker Endpoints is to **prevent broker connection failures** and **provide stable service to users** when Pod brokers are abnormally terminated or stopped.

### Previous Issues
- Service forwards traffic to Pod even when broker process is in abnormal state
- Connection failures occur when users attempt to connect to abnormal brokers
- Service interruption when Pod is in Running state but broker is not actually operating

### Broker Endpoint Solution
1. **Real-time broker status monitoring**: Continuously monitors broker process status inside Pod
2. **Dynamic Endpoint management**: Excludes Pod from Service traffic when broker is not Ready
3. **Automatic recovery**: Automatically re-includes in Service traffic when broker normalizes

### User Experience Improvement
- **Prevent connection failures**: Only Ready brokers accessible through Service
- **Automatic failure recovery**: Automatically switches traffic to other normal brokers when broker fails
- **Transparent service**: Users can use service without recognizing broker failures

## Key Features

### 1. Broker Status Monitoring
- Real-time verification of CUBRID broker process status inside each Pod
- Validates broker port listening status through `netstat` command
- Comprehensive judgment of Pod execution status and container status

### 2. Service Traffic Control
- Registers IP addresses of Pods with normally operating brokers in `Addresses` to allow Service traffic
- Registers IP addresses of Pods with abnormal broker status in `NotReadyAddresses` to block Service traffic
- Prevents users from connecting to abnormal brokers

### 3. Automatic Recovery and Retry
- Reconfirms broker status every 10 seconds to support automatic recovery
- Automatically detects broker status changes when Pod restarts
- Automatically re-includes in Service traffic when broker normalizes

## Pod Status Classification

### 1. Active Pod
- Pod is in Running state
- All containers are in Running state
- Has valid Pod IP address
- Broker port is in listening state

### 2. Not Ready Pod
- Pod is in Running state but broker port is not listening
- Error occurs during Pod status check
- Has valid Pod IP address but broker is abnormal

### 3. Excluded Pod
- Pod IP address is not valid
- Pod is not in Running state
- Containers are not operating normally

## Endpoint Structure

Structure of generated Endpoint resource:

```yaml
apiVersion: v1
kind: Endpoints
metadata:
  name: {broker-name}
  namespace: {cubriddb-namespace}
  labels:
    app: {cubriddb-name}
    broker: {broker-name}
  ownerReferences:
    - apiVersion: k8s.cubrid.com/v1
      kind: CubridDB
      name: {cubriddb-name}
      uid: {cubriddb-uid}
subsets:
  - addresses:
      - ip: {active-pod-ip}
        nodeName: {node-name}
        targetRef:
          kind: Pod
          name: {pod-name}
          namespace: {namespace}
          uid: {pod-uid}
    notReadyAddresses:
      - ip: {not-ready-pod-ip}
        nodeName: {node-name}
        targetRef:
          kind: Pod
          name: {pod-name}
          namespace: {namespace}
          uid: {pod-uid}
    ports:
      - name: {broker-name}
        port: {broker-port}
        protocol: TCP
```

## Considerations

### 1. Retry Interval
- Default retry interval: 10 seconds

### 2. Resource Usage
- Overhead from executing commands inside Pod
- Status verification through network communication

## Related Resources

- **CubridDB**: Resource to be monitored
- **Pod**: Target for broker status verification
- **Endpoints**: Resource to be created/managed
- **Service**: Service connected to Endpoint

## Notes

- Broker port verification depends on `netstat` command
- Requires `exec` permission for executing commands inside Pod
- Endpoint updates are performed only when there are changes
- Automatic re-verification performed when broker configuration changes
- Fundamentally prevents user connection failures through Service traffic control
- Extends Kubernetes default Endpoint behavior to provide broker-specific functionality 