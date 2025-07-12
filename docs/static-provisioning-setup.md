# 정적 프로비저닝 설정 가이드

## 개요
정적 프로비저닝을 사용하기 위해서는 각 워커 노드에 필요한 디렉토리를 미리 생성하고 적절한 권한을 설정해야 합니다.

## 필수 디렉토리 구조

### 1. 디렉토리 생성
각 워커 노드에서 다음 디렉토리들을 생성해야 합니다:

```bash
# 각 워커 노드에서 실행
sudo mkdir -p /data/cubrid/{database-0,database-1,logs-0,logs-1,conf-0,conf-1,backup-0,backup-1}
```

### 2. 권한 설정
생성된 디렉토리들의 소유자와 권한을 설정합니다:

```bash
# 각 워커 노드에서 실행
sudo chown -R 1000:1000 /data/cubrid/
sudo chmod -R 755 /data/cubrid/
```

### 3. 디렉토리 구조 확인
```bash
ls -la /data/cubrid/
```

예상 출력:
```
drwxr-xr-x. 8 cubrid cubrid 68 Jul  5 00:00 .
drwxr-xr-x. 3 root   root   20 Jul  4 23:21 ..
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 backup-0
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 backup-1
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 conf-0
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 conf-1
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 database-0
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 database-1
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 logs-0
drwxr-xr-x. 2 cubrid cubrid  6 Jul  4 23:21 logs-1
```

## 배포 순서

1. **디렉토리 준비** (위의 1-3단계)
2. **PV 생성**:
   ```bash
   kubectl apply -f pvs-for-static-provisioning.yaml
   ```
3. **CubridDB 배포**:
   ```bash
   kubectl apply -f cubrid_static_provisioning.yaml
   ```

## 문제 해결

### 권한 에러가 발생하는 경우
```bash
# Pod 로그 확인
kubectl logs <pod-name> -c init-recovery-conf

# 노드의 디렉토리 권한 확인
ls -la /data/cubrid/
```

### 특정 노드에서만 실행하고 싶은 경우
PV에 `nodeAffinity`를 추가하여 특정 노드에만 스케줄링할 수 있습니다:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: cubrid-database-pv-0
spec:
  # ... 기존 설정 ...
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - k8s-work3  # 특정 노드 이름
```

## 주의사항

1. **hostPath PV는 mountOptions를 지원하지 않습니다**
2. **각 노드마다 동일한 디렉토리 구조가 필요합니다**
3. **권한은 반드시 1000:1000 (cubrid:cubrid)이어야 합니다**
4. **StatefulSet의 각 replica는 서로 다른 PV를 사용합니다** 