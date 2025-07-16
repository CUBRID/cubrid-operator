# CUBRID Operator CMS 서비스 접속 가이드

## 개요

CUBRID Operator는 CMS(CUBRID Manager Server)에 접속하는 두 가지 방법을 제공합니다:

### 서비스 구성 방식

- **NodePort 서비스**: `cmsService.type: NodePort`로 설정 시 사용
- **Ingress 서비스**: `cmsService.type: Ingress`로 설정 시 사용

접속 방법은 `cmsService.type` 설정에 따라 결정되며, HA 모드 여부와 관계없이 두 방법 모두 사용할 수 있습니다.

## NodePort 서비스를 이용한 CMS 접속

### 개요
NodePort 서비스를 사용하여 CMS에 접속하는 방법입니다. 이 방법은 추가 인프라 없이 간단하게 CMS에 접속할 수 있습니다.

### 1. CR 설정 예시

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid-db
spec:
  cmsService:
    type: NodePort
    startPort: 31000
    port: 8001
  # ... 기타 설정
```

### 2. 생성되는 서비스

Operator가 자동으로 생성하는 NodePort 서비스:
- **서비스 이름**: `{cr-name}-{namespace}-cms-svc`
- **포트**: CMS 포트 (기본값: 8001)
- **NodePort**: startPort부터 순차적으로 할당

### 3. 접속 방법

```bash
# 서비스 확인
kubectl get svc | grep cms-svc

# 예시 출력
# NAME                    TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
# my-cubrid-db-cms-svc    NodePort   10.96.1.100    <none>        8001:31000/TCP   5m
```

**접속 URL**: `http://<node-ip>:<nodeport>`
- `<node-ip>`: Kubernetes 노드의 IP 주소
- `<nodeport>`: 할당된 NodePort 번호 (예: 31000)

### 4. HA 모드에서의 NodePort 서비스

HA 모드에서 NodePort를 사용할 경우, 각 파드별로 개별 NodePort 서비스가 생성됩니다:

```bash
# HA 모드에서 생성되는 서비스들
kubectl get svc | grep cms-svc

# 예시 출력
# NAME                    TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
# ha-cubrid-0-cms-svc     NodePort   10.96.1.101    <none>        8001:31000/TCP   5m
# ha-cubrid-1-cms-svc     NodePort   10.96.1.102    <none>        8001:31001/TCP   5m
# ha-cubrid-2-cms-svc     NodePort   10.96.1.103    <none>        8001:31002/TCP   5m
```

각 파드별 접속 URL:
- `http://<node-ip>:31000` (첫 번째 파드)
- `http://<node-ip>:31001` (두 번째 파드)
- `http://<node-ip>:31002` (세 번째 파드)

### 5. CUBRID Admin에서 CMS 접속

CUBRID Admin을 사용하여 CMS에 접속할 수 있습니다. CUBRID Admin에서 "서버 등록" 또는 "연결 추가" 기능을 통해 CMS 서버 정보를 입력하세요.

![CUBRID Admin CMS 접속](../assets/cubrid-admin-cms-connect.png)

**접속 정보 입력 예시**:
- **서버 이름**: `my-cubrid-db-cms`
- **호스트**: `<node-ip>` (Kubernetes 노드 IP)
- **포트**: `<nodeport>` (할당된 NodePort 번호, 예: 31000)
- **사용자**: CMS 접속용 사용자 계정
- **비밀번호**: CMS 접속용 비밀번호

## Ingress를 이용한 CMS 접속

Ingress를 사용하여 CMS에 접속하는 방법입니다. 이 방법은 도메인 기반 접근, SSL/TLS 지원, 로드밸런싱 등의 장점을 제공합니다.

### 사전 요구사항

#### 1. Ingress Controller 설치
클러스터에 Ingress Controller가 설치되어 있어야 합니다.

```bash
# nginx-ingress 설치 예시
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml

# 설치 확인
kubectl get pods -n ingress-nginx
```

### 2. DNS 설정
Ingress에서 사용할 도메인 이름에 대한 DNS 설정이 필요합니다.

### 3. Ingress Controller 포트 확인
Ingress Controller의 HTTPS 포트 번호를 확인해야 합니다.

```bash
# Ingress Controller Service 확인
kubectl get svc -n ingress-nginx

# 예시 출력
# NAME                                 TYPE           CLUSTER-IP       EXTERNAL-IP   PORT(S)                      AGE
# ingress-nginx-controller             LoadBalancer   10.96.1.100     192.168.1.100 80:30080/TCP,443:30443/TCP   5m
# ingress-nginx-controller-admission   ClusterIP      10.96.1.101     <none>        443/TCP                      5m
```

위 예시에서 HTTPS 포트는 `30443`입니다.

#### 포트 확인 방법

```bash
# 방법 1: Service 상세 정보 확인
kubectl describe svc ingress-nginx-controller -n ingress-nginx

# 방법 2: YAML 형태로 확인
kubectl get svc ingress-nginx-controller -n ingress-nginx -o yaml

# 방법 3: 포트만 추출
kubectl get svc ingress-nginx-controller -n ingress-nginx -o jsonpath='{.spec.ports[?(@.name=="https")].nodePort}'
```

## 호스트명 규칙

CUBRID Operator는 다음과 같은 호스트명 규칙을 사용합니다:

```
{pod-name}-{namespace}-{cms.cubrid.com}
```

### 호스트명 구성 요소

- **pod-name**: CUBRID 파드의 이름 (예: `cubrid-ha-0`, `cubrid-ha-1`)
- **namespace**: CUBRID가 배포된 네임스페이스 (예: `default`, `cubrid`)
- **cms.cubrid.com**: 고정 문자열 `cms.cubrid.com`

### 호스트명 예시

```
# 기본 네임스페이스의 첫 번째 파드
cubrid-ha-0.default.cms.cubrid.com

# cubrid 네임스페이스의 두 번째 파드  
cubrid-ha-1.cubrid.cms.cubrid.com

# production 네임스페이스의 마스터 파드
cubrid-ha-0.production.cms.cubrid.com
```

## Ingress를 이용한 CMS 서비스

`cmsService.type: Ingress`로 설정하면, Operator가 자동으로 Ingress 리소스를 생성하여 CMS 서비스에 도메인 기반으로 접근할 수 있습니다. HA 모드와 단일 서버 모드 모두에서 사용 가능합니다.

### 1. CR 예시

#### 단일 서버 모드
```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: cubrid-single
  namespace: default
spec:
  cmsService:
    type: Ingress
    port: 8001
  # ... 기타 설정
```

#### HA 모드
```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: cubrid-ha
  namespace: default
spec:
  replication:
    enable: true
    replicas: 3
    hamodeType:
      type: master-slave
  cmsService:
    type: Ingress
    port: 8001
```

### 2. 자동 생성되는 Ingress 및 호스트 구조

Operator가 자동으로 생성하는 Ingress의 이름 규칙:
- `{cr-name}-{namespace}-{cms-ingress}`

#### 단일 서버 모드
단일 서버 모드에서는 하나의 호스트만 생성됩니다:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: cubrid-single-default-cms-ingress
  namespace: default
spec:
  rules:
  - host: cubrid-single.default.cms.cubrid.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: cubrid-single-default-cms-svc
            port:
              number: 8001
```

#### HA 모드
HA 모드에서는 각 파드별로 CMS에 접속하기 위한 호스트가 자동으로 추가됩니다:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: cubrid-ha-default-cms-ingress
  namespace: default
spec:
  rules:
  - host: cubrid-ha-0.default.cms.cubrid.com  # cubrid-ha-0
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: cubrid-ha-0-default-cms-svc
            port:
              number: 8001
  - host: cubrid-ha-1.default.cms.cubrid.com  # cubrid-ha-1
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: cubrid-ha-1-default-cms-svc
            port:
              number: 8001
  - host: cubrid-ha-2.default.cms.cubrid.com  # cubrid-ha-2
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: cubrid-ha-2-default-cms-svc
            port:
              number: 8001
```


### 3. Ingress 리소스의 인증서(SSL) 관련 어노테이션

CUBRID CMS는 HTTPS(SSL)로 통신하므로, Operator가 생성하는 Ingress에는 다음과 같은 어노테이션이 자동으로 추가됩니다.

```yaml
annotations:
  nginx.ingress.kubernetes.io/backend-protocol: HTTPS
  nginx.ingress.kubernetes.io/ssl-passthrough: "true"
```

- `nginx.ingress.kubernetes.io/backend-protocol: HTTPS`
  → 백엔드(CMS 서비스)와 Ingress 간 통신이 HTTPS임을 명시합니다.
- `nginx.ingress.kubernetes.io/ssl-passthrough: "true"`
  → 클라이언트와 CMS 간의 SSL 연결을 그대로 전달(패스스루)합니다.

> **참고:**
> CUBRID Operator가 생성하는 Ingress는 `ssl-passthrough` 설정을 사용하여,
> 클라이언트의 TLS/SSL 연결을 Ingress가 종료하지 않고 CMS로 그대로 전달합니다.
> 따라서 Ingress에서 별도의 TLS/SSL 인증서(secret) 설정은 필요하지 않으며,
> CMS가 직접 인증서를 관리하고 HTTPS 통신을 처리합니다.

#### 실제 Operator가 생성한 Ingress 예시

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
  name: cubrid-ha-default-cms-ingress
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - host: cubrid-ha-0.default.cms.cubrid.com
    http:
      paths:
      - backend:
          service:
            name: cubrid-ha-0-default-cms-svc
            port:
              number: 8001
        path: /
        pathType: Prefix
  - host: cubrid-ha-1.default.cms.cubrid.com
    http:
      paths:
      - backend:
          service:
            name: cubrid-ha-1-default-cms-svc
            port:
              number: 8001
        path: /
        pathType: Prefix
```
### 4. CUBRID Admin에서 CMS 접속

CUBRID Admin을 사용하여 Ingress를 통해 CMS에 접속할 수 있습니다. 도메인 기반 접근이 가능하므로 더 직관적인 접속이 가능합니다.

**접속 정보 입력 예시**:

- **서버 이름**: `cubrid-ha-0-cms` (첫 번째 파드)
- **호스트**: `cubrid-ha-0.default.cms.cubrid.com`
- **포트**: `30443` (HTTPS 기본 포트)
- **사용자**: CMS 접속용 사용자 계정
- **비밀번호**: CMS 접속용 비밀번호

## 모니터링 및 로깅

### 1. Ingress 로그 확인

```bash
# Ingress Controller 로그 확인
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# 특정 호스트에 대한 로그 필터링
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller | grep "cubrid-ha-0.default.cms"
```

### 2. CMS 서비스 상태 확인

```bash
# CMS 서비스 목록 확인
kubectl get svc -l app=cubrid

# 특정 CMS 서비스 상세 정보
kubectl describe svc cubrid-ha-0-default-cms-svc
```

### 3. 엔드포인트 확인

```bash
# CMS 서비스 엔드포인트 확인
kubectl get endpoints cubrid-ha-0-default-cms-svc

# 파드 연결 상태 확인
kubectl get pods -l app=cubrid-ha
```

## 문제 해결

### 1. Ingress가 작동하지 않는 경우

```bash
# Ingress 상태 확인
kubectl get ingress
kubectl describe ingress cubrid-ha-default-cms-ingress

# Ingress Controller 상태 확인
kubectl get pods -n ingress-nginx
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# Ingress Controller 포트 확인
kubectl get svc ingress-nginx-controller -n ingress-nginx

### 2. CMS 서비스에 연결할 수 없는 경우

```bash
# CMS 서비스 상태 확인
kubectl get svc | grep cms-svc

# 서비스 엔드포인트 확인
kubectl get endpoints | grep cms-svc

# 파드 상태 확인
kubectl get pods -l app=cubrid-ha
```

### 3. 포트 충돌 문제

```bash
# 포트 사용 현황 확인
kubectl get svc -o wide | grep 8001

# 포트 충돌 해결을 위해 다른 포트 사용
```


### 배포 및 확인

```bash
# CubridDB CR 배포
kubectl apply -f cubrid-ha.yaml

# 자동 생성된 Ingress 확인
kubectl get ingress {CR-Name}-{Namespace}-cms-ingress

# Ingress 상세 정보 확인
kubectl describe ingress {CR-Name}-{Namespace}-cms-ingress

# 생성된 호스트 확인
kubectl get ingress {CR-Name}-{Namespace}-cms-ingress -o jsonpath='{.spec.rules[*].host}'
```

### 생성되는 리소스

Operator가 자동으로 생성하는 리소스들:

1. **Ingress**: 
2. **CMS 서비스들**: 
   - `cubrid-ha-0-default-cms-svc`
   - `cubrid-ha-1-default-cms-svc`
   - `cubrid-ha-2-default-cms-svc`
3. **호스트들**:
   - `cubrid-ha-0.default.cms.cubrid.com`
   - `cubrid-ha-1.default.cms.cubrid.com`
   - `cubrid-ha-2.default.cms.cubrid.com`

이 가이드를 따라 Ingress를 통해 CUBRID CMS 서비스에 안전하고 효율적으로 접근할 수 있습니다.

