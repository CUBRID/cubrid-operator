# CUBRID Operator 서비스 관리

## 개요

CUBRID Operator의 서비스 관리에 대해 설명합니다.

CUBRID Operator는 CUBRID 데이터베이스 인스턴스를 위한 다양한 서비스를 관리합니다. 모든 서비스는 **CR 기반 관리**를 통해 운영되며, Custom Resource (CR) 명세를 통해 구성하고 수정해야 합니다. `kubectl edit svc`를 통한 직접 수정은 권장되지 않습니다.

## 서비스 타입

### 1. 브로커 서비스 (Broker Services)
- **목적**: CUBRID 브로커 포트를 클라이언트 연결용으로 노출
- **구성**: `spec.broker[]` 배열에 정의
- **관리**: CR 기반 관리
- **수정**: `kubectl edit cubriddb <name>` 사용

### 2. 헤드리스 서비스 (Headless Service)
- **목적**: HA 모드에서 StatefulSet 파드의 안정적인 네트워크 식별자 제공
- **구성**: 자동 생성 (HA 모드 활성화 시)
- **관리**: CR 기반 관리
- **수정**: `kubectl edit cubriddb <name>` 사용

### 3. CMS 서비스
- **목적**: CUBRID Manager Server를 관리용으로 노출
- **구성**: `spec.cmsService.type`에 따라 서비스 타입 결정
- **관리**: CR 기반 관리
- **타입**:
  - **NodePort**: `type: "NodePort"` - NodePort 서비스 생성
  - **Ingress**: `type: "Ingress"` - Ingress CMS 서비스 + Ingress 리소스 생성
- **수정**: `kubectl edit cubriddb <name>` 사용
- **제한사항**: 서비스 타입은 생성 후 변경할 수 없음 (immutable)

## 서비스 관리 원칙

### CR 기반 관리 서비스
모든 서비스는 Custom Resource 명세를 통해 관리됩니다. 이는 다음을 보장합니다:

1. **일관성**: 모든 서비스 구성이 한 곳에 저장됨
2. **버전 관리**: 서비스 변경사항을 Git을 통해 추적 가능
3. **자동화**: 서비스가 CR 상태와 일치하도록 자동 조정됨
4. **보호**: 서비스에 대한 직접 수정이 자동으로 되돌려짐

### 자동 정리
- **소유자 참조**: 
  - 브로커, 헤드리스, CMS 서비스는 CubridDB 리소스에 대한 소유자 참조를 가짐
  - Ingress CMS 서비스는 해당 Pod 리소스에 대한 소유자 참조를 가짐
- **파드 생명주기**: Ingress CMS 서비스는 새 파드에 대해 자동 생성되고, 파드 제거 시 삭제됨
- **리소스 정리**: CubridDB가 삭제되면 모든 관련 서비스가 자동으로 정리됨
- **파드 삭제**: 개별 파드가 삭제되면 관련 Ingress CMS 서비스가 자동으로 정리됨

## 구성 예시

### 브로커 서비스 구성
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

### CMS 서비스 구성
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

### HA 포트 구성
```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid
spec:
  replication:
    enable: true
    replicas: 2
    haPort: 59901  # HA 모드에서만 사용되는 포트
    hamodeType:
      type: master-slave
```

## 서비스 수정
서비스 수정은 CR을 통해서만 가능 합니다. 서비스를 직접 수정하면 이전 설정으로 원복됩니다.

### CR을 통한 수정
```bash
kubectl edit cubriddb my-cubrid
```

### 간단한 설정 예시

#### 브로커 서비스 포트 변경
```bash
# 1. CR 편집
kubectl edit cubriddb my-cubrid

# 2. spec.broker 섹션에서 포트 수정
spec:
  broker:
    - name: query-editor
      port: 33000
      servicePort: 30001  # 30000에서 30001로 변경
      serviceType: NodePort
```

#### CMS 서비스 타입 변경 (제한됨)
```bash
# CMS Service 타입은 생성 후 변경할 수 없습니다
# NodePort → Ingress 또는 Ingress → NodePort 변경 시도 시 오류 발생

# 잘못된 예시 (오류 발생)
spec:
  cmsService:
    type: Ingress  # 기존 NodePort에서 변경 시도 → 오류

# 올바른 방법: 새로운 CubridDB 인스턴스 생성
# 1. 기존 인스턴스 백업
# 2. 새로운 인스턴스 생성 (원하는 타입으로)
# 3. 데이터 마이그레이션
```

#### HA 포트 변경
```bash
# 1. CR 편집
kubectl edit cubriddb my-cubrid

# 2. spec.replication 섹션에서 포트 수정
spec:
  replication:
    enable: true
    replicas: 2
    haPort: 59902  # 59901에서 59902로 변경
```

#### Replicas 수 변경 (서비스 개수 자동 조정)
```bash
# 1. CR 편집
kubectl edit cubriddb my-cubrid

# 2. spec.replication 섹션에서 replicas 수정
spec:
  replication:
    enable: true
    replicas: 3  # 2에서 3으로 변경 (CMS 서비스도 3개로 자동 조정)
```

## 서비스 네이밍 규칙

### 브로커 서비스
- **형식**: `{broker.name}`
- **예시**: `query-editor`, `broker1`

### CMS NodePort 서비스
- **형식**: `{cubriddb-name}-{namespace}-cms-{pod-index}`
- **예시**: `my-cubrid-default-cms-0`, `my-cubrid-default-cms-1`

### Ingress CMS 서비스
- **형식**: `{pod-name}-{namespace}-cms-svc`
- **예시**: `my-cubrid-0-default-cms-svc`, `my-cubrid-1-default-cms-svc`

### 헤드리스 서비스
- **형식**: `{cubriddb-name}-headless`
- **예시**: `my-cubrid-headless`

## 서비스 타입별 상세 정보

### 브로커 서비스
- **서비스 타입**: NodePort (현재 지원)
- **포트 구성**: 브로커 포트 → NodePort 매핑
- **자동 생성**: CR의 `spec.broker[]` 배열 기반
- **소유자**: CubridDB 리소스

### CMS NodePort 서비스
- **서비스 타입**: NodePort
- **포트 구성**: CMS 포트 → NodePort 매핑
- **자동 생성**: 파드별로 생성
- **소유자**: 해당 파드
- **조건**: `spec.cmsService.type`이 NodePort 일때 생성

### Ingress CMS 서비스
- **서비스 타입**: ClusterIP (Headless)
- **포트 구성**: CMS 포트 사용
- **자동 생성**: 파드별로 생성
- **소유자**: 해당 파드
- **조건**: `spec.cmsService.type`이 Ingress 일때 생성

### 헤드리스 서비스
- **서비스 타입**: ClusterIP (Headless)
- **포트 구성**: HA 포트 사용
- **자동 생성**: HA 모드 활성화 시
- **소유자**: CubridDB 리소스
- **조건**: `spec.replication.enable: true`일 때만 생성 