# BrokerEndpointReconciler

## 개요

CUBRID Operator의 BrokerEndpointReconciler에 대해 설명합니다.

`BrokerEndpointReconciler`는 CUBRID Operator의 핵심 컴포넌트 중 하나로, CUBRID 데이터베이스의 브로커 서비스 상태를 모니터링하고 Kubernetes Endpoint 리소스를 동적으로 관리하는 컨트롤러입니다.

## Broker Endpoint 생성 목적

Broker Endpoint를 생성한 주요 이유는 Pod의 Broker가 비정상 종료 되었거나, 중지 되어 있는 경우, **브로커 접속 실패를 방지**하고 **사용자에게 안정적인 서비스를 제공**하기 위함입니다.

### 기존 문제점
- 브로커 프로세스가 비정상 상태여도 Service가 해당 Pod로 트래픽을 전달
- 사용자가 비정상 브로커에 접속 시도 시 연결 실패 발생
- Pod는 Running 상태이지만 브로커가 실제로 동작하지 않는 상황에서의 서비스 중단

### Broker Endpoint의 해결책
1. **실시간 브로커 상태 감시**: Pod 내부의 브로커 프로세스 상태를 지속적으로 모니터링
2. **동적 Endpoint 관리**: 브로커가 Ready 상태가 아닌 경우 해당 Pod를 Service 트래픽에서 제외
3. **자동 복구**: 브로커가 정상화되면 자동으로 Service 트래픽에 다시 포함

### 사용자 경험 개선
- **연결 실패 방지**: Ready 상태의 브로커만 Service를 통해 접근 가능
- **자동 장애 복구**: 브로커 장애 시 자동으로 다른 정상 브로커로 트래픽 전환
- **투명한 서비스**: 사용자는 브로커 장애를 인지하지 못하고 서비스 이용 가능

## 주요 기능

### 1. 브로커 상태 모니터링
- 각 Pod 내부의 CUBRID 브로커 프로세스 상태를 실시간으로 확인
- 브로커 포트의 리스닝 상태를 `netstat` 명령어를 통해 검증
- Pod의 실행 상태와 컨테이너 상태를 종합적으로 판단

### 2. Service 트래픽 제어
- 브로커가 정상 동작하는 Pod의 IP 주소를 `Addresses`에 등록하여 Service 트래픽 허용
- 브로커가 비정상 상태인 Pod의 IP 주소를 `NotReadyAddresses`에 등록하여 Service 트래픽 차단
- 사용자가 비정상 브로커에 접속하는 것을 방지

### 3. 자동 복구 및 재시도
- 10초마다 브로커 상태를 재확인하여 자동 복구 지원
- Pod 재시작 시 브로커 상태 변화를 자동으로 감지
- 브로커 정상화 시 자동으로 Service 트래픽에 다시 포함


## Pod 상태 분류

### 1. 활성 Pod (Active)
- Pod가 Running 상태
- 모든 컨테이너가 Running 상태
- 유효한 Pod IP 주소 보유
- 브로커 포트가 리스닝 상태

### 2. 비활성 Pod (Not Ready)
- Pod가 Running 상태이지만 브로커 포트가 리스닝하지 않음
- Pod 상태 확인 중 오류 발생
- 유효한 Pod IP 주소는 보유하지만 브로커가 비정상

### 3. 제외 Pod
- Pod IP 주소가 유효하지 않음
- Pod가 Running 상태가 아님
- 컨테이너가 정상 동작하지 않음

## Endpoint 구조

생성되는 Endpoint 리소스의 구조:

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

## 고려사항

### 1. 재시도 간격
- 기본 재시도 간격: 10초

### 2. 리소스 사용량
- Pod 내부 명령어 실행으로 인한 오버헤드
- 네트워크 통신을 통한 상태 확인


## 관련 리소스

- **CubridDB**: 모니터링 대상 리소스
- **Pod**: 브로커 상태 확인 대상
- **Endpoints**: 생성/관리 대상 리소스
- **Service**: Endpoint와 연결되는 서비스

## 참고 사항

- 브로커 포트 확인은 `netstat` 명령어에 의존
- Pod 내부 명령어 실행을 위해 `exec` 권한 필요
- Endpoint 업데이트는 변경사항이 있을 때만 수행
- 브로커 설정 변경 시 자동으로 재검증 수행
- Service 트래픽 제어를 통해 사용자 접속 실패를 근본적으로 방지
- Kubernetes의 기본 Endpoint 동작을 확장하여 브로커 특화 기능 제공 