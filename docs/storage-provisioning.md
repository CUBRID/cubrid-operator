# CUBRID Operator 스토리지 프로비저닝 가이드

CUBRID Operator는 두 가지 스토리지 프로비저닝 방식을 지원합니다: **동적 프로비저닝(Dynamic Provisioning)**과 **정적 프로비저닝(Static Provisioning)**.

## 목차

- [개요](#개요)
- [동적 프로비저닝](#동적-프로비저닝)
- [정적 프로비저닝](#정적-프로비저닝)
- [프로비저닝 방식 선택](#프로비저닝-방식-선택)
- [예시 파일](#예시-파일)
- [문제 해결](#문제-해결)

## 개요

### 동적 프로비저닝 (Dynamic Provisioning)
- StorageClass를 사용하여 자동으로 PersistentVolume(PV)을 생성
- PVC 생성 시 자동으로 PV가 프로비저닝됨
- 클라우드 환경이나 스토리지 시스템에서 권장되는 방식

### 정적 프로비저닝 (Static Provisioning)
- 관리자가 사전에 PV를 생성하고 관리
- PVC는 기존 PV에 바인딩됨
- 온프레미스 환경이나 특정 스토리지 요구사항이 있는 경우 사용

## 동적 프로비저닝

### 특징
- **자동화**: PVC 생성 시 자동으로 PV 생성
- **확장성**: 필요에 따라 자동으로 스토리지 확장
- **관리 편의성**: 수동 PV 관리 불필요

### 설정 방법

#### 1. StorageClass 확인
먼저 클러스터에서 사용 가능한 StorageClass를 확인합니다:

```bash
kubectl get storageclass
```

일반적인 StorageClass 예시:
- `longhorn` (Longhorn 스토리지)
- `standard` (기본 스토리지)
- `fast-ssd` (고성능 SSD)

#### 2. CubridDB 설정

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
          storageClassName: longhorn  # 동적 프로비저닝을 위한 StorageClass
  broker:
    - name: broker-svc
      port: 30000
      serviceType: ClusterIP
```

#### 3. 적용 및 확인

```bash
# CubridDB 생성
kubectl apply -f cubrid-dynamic.yaml

# PVC 상태 확인
kubectl get pvc

# PV 상태 확인
kubectl get pv

# 파드 상태 확인
kubectl get pods -l app=cubriddb
```

### 동적 프로비저닝 예시 파일

`config/samples/cubrid-dynamic-provisioning.yaml` 파일을 참조하세요.

## 정적 프로비저닝

### 특징
- **수동 관리**: 관리자가 PV를 사전에 생성
- **제어 가능**: 정확한 스토리지 위치와 특성 제어
- **성능 예측**: 미리 정의된 스토리지 성능 보장

### 설정 방법

#### 1. PersistentVolume 생성

먼저 사용할 PV를 생성합니다:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: cubrid-db-pv-0
  labels:
    type: cubrid-database
    app: cubrid-db
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""  # 빈 문자열 = 정적 프로비저닝
  hostPath:
    path: /data/cubrid/db-0
    type: DirectoryOrCreate
```

#### 2. 디렉토리 생성 및 권한 설정

노드에서 디렉토리를 생성하고 권한을 설정합니다:

```bash
# 각 노드에서 실행
sudo mkdir -p /data/cubrid/db-0
sudo chown 1000:1000 /data/cubrid/db-0
sudo chmod 755 /data/cubrid/db-0
```

#### 3. CubridDB 설정

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
          storageClassName: ""  # 빈 문자열 = 정적 프로비저닝
          selector:
            matchLabels:
              type: cubrid-database
              app: cubrid-static
  broker:
    - name: broker-svc
      port: 30000
      serviceType: ClusterIP
```

#### 4. 적용 순서

```bash
# 1. PV 생성
kubectl apply -f cubrid-pv.yaml

# 2. PV 상태 확인
kubectl get pv

# 3. CubridDB 생성
kubectl apply -f cubrid-static.yaml

# 4. PVC 상태 확인
kubectl get pvc

# 5. 파드 상태 확인
kubectl get pods -l app=cubriddb
```

### 정적 프로비저닝 예시 파일

- `config/samples/cubrid-static-provisioning.yaml`: CubridDB 설정
- `config/samples/cubrid-static-pv.yaml`: PV 설정

## 프로비저닝 방식 선택

### 동적 프로비저닝을 선택하는 경우
- 클라우드 환경 (AWS, GCP, Azure)
- 스토리지 시스템이 자동 프로비저닝을 지원
- 개발/테스트 환경
- 빠른 배포가 필요한 경우

### 정적 프로비저닝을 선택하는 경우
- 온프레미스 환경
- 특정 스토리지 위치나 성능 요구사항
- 프로덕션 환경에서 정확한 제어 필요
- 기존 스토리지 인프라 활용

## 예시 파일

### 동적 프로비저닝 예시

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

### 정적 프로비저닝 예시

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
  name: cubrid-db-pv-0
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
    path: /data/cubrid/db-0
    type: DirectoryOrCreate
```

## 문제 해결

### 동적 프로비저닝 문제

#### 1. StorageClass 없음
```
Error: no storage class is set on request and no default can be found
```

**해결 방법**:
```bash
# 사용 가능한 StorageClass 확인
kubectl get storageclass

# 기본 StorageClass 설정
kubectl patch storageclass <storage-class-name> -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

#### 2. 볼륨 프로비저닝 실패
```
Failed to provision volume with StorageClass "longhorn"
```

**해결 방법**:
- 스토리지 시스템 상태 확인
- 디스크 공간 확인
- 스토리지 클래스 설정 확인

### 정적 프로비저닝 문제

#### 1. 권한 오류
```
Permission denied: /opt/cubrid/databases
```

**해결 방법**:
```bash
# 노드에서 디렉토리 권한 설정
sudo mkdir -p /data/cubrid/db-0
sudo chown 1000:1000 /data/cubrid/db-0
sudo chmod 755 /data/cubrid/db-0
```

#### 2. PVC 바인딩 실패
```
PVC stuck in Pending state
```

**해결 방법**:
- PV 라벨과 PVC 셀렉터 일치 확인
- PV 상태 확인 (`kubectl get pv`)
- 스토리지 크기 일치 확인

#### 3. mountOptions 오류
```
mountOptions not supported for hostPath volumes
```

**해결 방법**:
- hostPath 볼륨에서는 mountOptions 제거
- 다른 볼륨 타입 사용 고려

### 일반적인 문제 해결 명령어

```bash
# PVC 상태 확인
kubectl get pvc
kubectl describe pvc <pvc-name>

# PV 상태 확인
kubectl get pv
kubectl describe pv <pv-name>

# 파드 상태 확인
kubectl get pods -l app=cubriddb
kubectl describe pod <pod-name>

# 파드 로그 확인
kubectl logs <pod-name>

# 이벤트 확인
kubectl get events --sort-by='.lastTimestamp'
```

## 추가 리소스

- [Kubernetes Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [Storage Classes](https://kubernetes.io/docs/concepts/storage/storage-classes/)
- [API Reference](./api-reference.md)
- [예시 파일](../config/samples/) 