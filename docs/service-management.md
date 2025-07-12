# CUBRID Operator 서비스 관리

## 개요

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

### 3. CMS 서비스 (NodePort)
- **목적**: CUBRID Manager Server를 관리용으로 노출
- **구성**: `spec.cmsService`
- **관리**: CR 기반 관리
- **참고**: nginx ingress controller가 없을 때만 생성됨

### 4. Ingress CMS 서비스
- **목적**: Ingress 리소스의 백엔드 서비스 (nginx ingress controller가 있을 때)
- **구성**: `spec.cmsService.port`를 일관성 있게 사용
- **관리**: CR 기반 관리
- **자동 정리**: 기존 파드에 대해 서비스가 자동 생성되고, 파드 삭제 시 정리됨
- **수정**: `kubectl edit cubriddb <name>` 사용

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
    enabled: true
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
  haPort:
    port: 59901
```

## 서비스 수정

### ❌ 이렇게 하지 마세요
```bash
# 직접 서비스 수정 (되돌려짐)
kubectl edit svc my-cubrid-query-editor
```

### ✅ 이렇게 하세요
```bash
# CR을 통한 수정 (권장)
kubectl edit cubriddb my-cubrid
```

## 서비스 조정 (Reconciliation)

Operator는 다음 조정 패턴을 따릅니다:

1. **비교**: 원하는 상태 (CR에서) vs 현재 상태 (Kubernetes에서)
2. **업데이트**: 차이가 감지될 때만 수행
3. **보호**: 무단 직접 수정을 되돌림

### 조정 트리거
- CR 명세 변경
- 파드 생명주기 이벤트 (Ingress CMS 서비스용)
- Ingress controller 존재/부재
- 수동 조정 요청

## 서비스 보호 메커니즘

### CR 기반 관리 서비스 보호
Operator는 CR 기반 관리 서비스에 대해 다음과 같은 보호 메커니즘을 제공합니다:

1. **직접 수정 감지**: 사용자가 직접 서비스를 수정하면 감지
2. **자동 복구**: CR 명세에 맞게 서비스를 자동으로 복구
3. **경고 로그**: 무단 수정 시 경고 로그 생성

### 보호 대상 서비스
- 브로커 서비스 (`spec.broker[]`에 정의된 서비스)
- 헤드리스 서비스 (HA 모드에서 자동 생성)
- CMS NodePort 서비스 (`spec.cmsService`에 정의된 서비스)
- Ingress CMS 서비스 (파드별로 자동 생성)

## 문제 해결

### 서비스가 업데이트되지 않는 경우
CR 변경 후 서비스가 업데이트되지 않는다면:

1. 서비스가 CR 기반 관리인지 확인
2. CR 변경사항이 적용되었는지 확인
3. Operator 로그에서 조정 오류 확인
4. 직접 서비스 수정이 간섭하지 않는지 확인

### 고아 서비스
파드 삭제 후 서비스가 남아있는 경우:

1. 소유자 참조가 올바르게 설정되었는지 확인
2. 서비스가 올바른 CubridDB에 속하는지 확인
3. Operator 로그에서 정리 오류 확인

### 포트 충돌
포트 충돌로 인해 서비스 생성이 실패하는 경우:

1. CR 구성에서 NodePort 범위 확인
2. 다른 서비스가 같은 포트를 사용하지 않는지 확인
3. CR에서 포트 구성 조정

## 모범 사례

1. **항상 CR을 통해 서비스 수정**: `kubectl edit svc` 직접 사용 금지
2. **의미 있는 서비스 이름 사용**: 설명적인 브로커 이름 구성
3. **포트 범위 계획**: NodePort 범위를 계획하여 충돌 방지
4. **조정 모니터링**: Operator 로그에서 서비스 조정 상태 확인
5. **버전 관리**: CR 구성을 Git에 저장하여 변경사항 추적

## 사용자 관리 서비스에서 마이그레이션

기존 사용자 관리 서비스가 있다면:

1. **현재 구성 백업**: 현재 서비스 설정 문서화
2. **CR 업데이트**: CR 명세에 서비스 구성 추가
3. **변경사항 적용**: 업데이트된 CR 적용
4. **검증**: 서비스가 올바르게 조정되었는지 확인
5. **정리**: 수동 서비스 구성 제거

Operator는 자동으로 서비스를 CR 기반 관리 모드로 전환하고 직접 수정으로부터 보호합니다.

## 포트 관리

### 통합 CMS 포트
모든 CMS 관련 서비스는 단순성과 일관성을 위해 단일 포트 구성을 사용합니다:

- **소스**: `spec.cmsService.port`
- **사용 대상**:
  - CMS NodePort 서비스
  - Ingress CMS 서비스  
  - Ingress 백엔드 서비스

이를 통해 CR에서 CMS 포트를 변경하면 모든 관련 서비스가 일관되게 업데이트됩니다.

### 단순화된 구성
Operator는 중복을 피하고 일관성을 유지하기 위해 단일 포트 구성을 사용합니다:

```yaml
cmsService:
  enabled: true
  port: 8001        # ← 이 단일 포트가 모든 CMS 서비스에 사용됨
  startPort: 31000
```

모든 CMS 서비스가 같은 포트를 사용하므로 별도의 `ingressService` 구성이 필요하지 않습니다.

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

## 서비스 라벨링

모든 서비스는 다음과 같은 라벨을 가집니다:

```yaml
labels:
  app: {cubriddb-name}
  service: {service-type}  # broker, cms, headless
```

브로커 서비스의 경우 추가 라벨:
```yaml
labels:
  broker: {broker-name}
```

## 서비스 선택자 (Selector)

### 브로커 서비스
```yaml
selector:
  app: {cubriddb-name}
```

### CMS 서비스
```yaml
selector:
  statefulset.kubernetes.io/pod-name: {pod-name}
```

### 헤드리스 서비스
```yaml
selector:
  app: {cubriddb-name}
```

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
- **조건**: nginx ingress controller가 없을 때만 생성

### Ingress CMS 서비스
- **서비스 타입**: ClusterIP (Headless)
- **포트 구성**: CMS 포트 사용
- **자동 생성**: 파드별로 생성
- **소유자**: 해당 파드
- **조건**: nginx ingress controller가 있을 때 생성

### 헤드리스 서비스
- **서비스 타입**: ClusterIP (Headless)
- **포트 구성**: HA 포트 사용
- **자동 생성**: HA 모드 활성화 시
- **소유자**: CubridDB 리소스
- **조건**: `spec.replication.enable: true`일 때만 생성 