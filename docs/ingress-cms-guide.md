# CUBRID Operator Ingress CMS 서비스 접속 가이드

## 개요

CUBRID Operator는 **HA Mode에서만** Ingress 리소스를 통해 CMS(CUBRID Manager Server) 서비스에 접근할 수 있는 기능을 제공합니다. 단일 서버 구성에서는 NodePort Service를 사용하여 CMS에 접근합니다.

### 서비스 구성 방식

- **HA Mode**: Ingress + Ingress CMS Service (ClusterIP Headless)
- **단일 서버**: NodePort Service

이를 통해 사용자는 HA Mode에서는 도메인 이름을 사용하여, 단일 서버에서는 NodePort를 사용하여 CUBRID Manager Server(CMS)에 접속할 수 있습니다.

## 사전 요구사항

### 1. Ingress Controller 설치
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

- **pod-name**: CUBRID 파드의 이름 (예: `ha-ms-0`, `ha-ms-1`)
- **namespace**: CUBRID가 배포된 네임스페이스 (예: `default`, `cubrid`)
- **cms.cubrid.com**: 고정 문자열 `cms.cubrid.com`

### 호스트명 예시

```
# 기본 네임스페이스의 첫 번째 파드
ha-ms-0.default.cms.cubrid.com

# cubrid 네임스페이스의 두 번째 파드  
ha-ms-1.cubrid.cms.cubrid.com

# production 네임스페이스의 마스터 파드
ha-ms-0.production.cms.cubrid.com
```

## HA 모드에서의 Ingress CMS 서비스

HA 모드에서 CubridDB CR을 배포하면, Operator가 자동으로 Ingress 리소스를 생성하여 각 파드별 CMS 서비스에 도메인 기반으로 접근할 수 있습니다.

### 1. CR 예시

```yaml
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: ha-cubrid
  namespace: default
spec:
  replication:
    enable: true
    replicas: 3
    hamodeType:
      type: master-slave
  cmsService:
    enabled: true
    port: 8001
    startPort: 31000
```

### 2. 자동 생성되는 Ingress 및 호스트 구조

Operator가 자동으로 생성하는 Ingress의 이름 규칙:
- `{cr-name}-{namespace}-{cms-ingress}`

각 파드별로 CMS에 접속하기 위한 호스트가 자동으로 추가됩니다:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ha-cubrid-default-cms-ingress
  namespace: default
spec:
  rules:
  - host: ha-ms-0.default.cms.cubrid.com  # 마스터 파드
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ha-ms-0-default-cms-svc
            port:
              number: 8001
  - host: ha-ms-1.default.cms.cubrid.com  # 슬레이브 파드 1
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ha-ms-1-default-cms-svc
            port:
              number: 8001
  - host: ha-ms-2.default.cms.cubrid.com  # 슬레이브 파드 2
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ha-ms-2-default-cms-svc
            port:
              number: 8001
```

각 파드에 개별적으로 접속할 수 있습니다:
- **마스터 파드**: `http://ha-ms-0.default.cms.cubrid.com`
- **슬레이브 파드 1**: `http://ha-ms-1.default.cms.cubrid.com`
- **슬레이브 파드 2**: `http://ha-ms-2.default.cms.cubrid.com`

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
  name: ha-ms-default-cms-ingress
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - host: ha-ms-0.default.cms.cubrid.com
    http:
      paths:
      - backend:
          service:
            name: ha-ms-0-default-cms-svc
            port:
              number: 8001
        path: /
        pathType: Prefix
  - host: ha-ms-1.default.cms.cubrid.com
    http:
      paths:
      - backend:
          service:
            name: ha-ms-1-default-cms-svc
            port:
              number: 8001
        path: /
        pathType: Prefix
```

## 모니터링 및 로깅

### 1. Ingress 로그 확인

```bash
# Ingress Controller 로그 확인
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# 특정 호스트에 대한 로그 필터링
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller | grep "ha-ms-0.default.cms.cubrid.com"
```

### 2. CMS 서비스 상태 확인

```bash
# CMS 서비스 목록 확인
kubectl get svc -l app=cubrid-operator-webhook-dep

# 특정 CMS 서비스 상세 정보
kubectl describe svc ha-ms-0-default-cms-svc
```

### 3. 엔드포인트 확인

```bash
# CMS 서비스 엔드포인트 확인
kubectl get endpoints ha-ms-0-default-cms-svc

# 파드 연결 상태 확인
kubectl get pods -l app=cubrid-operator-webhook-dep
```

## 문제 해결

### 1. Ingress가 작동하지 않는 경우

```bash
# Ingress 상태 확인
kubectl get ingress
kubectl describe ingress cubrid-cms-ingress

# Ingress Controller 상태 확인
kubectl get pods -n ingress-nginx
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# Ingress Controller 포트 확인
kubectl get svc ingress-nginx-controller -n ingress-nginx

# 포트 연결 테스트
telnet <노드IP> <NodePort>
# 예시: telnet 192.168.1.100 30443
```

### 2. DNS 해결이 안 되는 경우

```bash
# DNS 해결 테스트
nslookup ha-ms-0.default.cms.cubrid.com

# 로컬 hosts 파일 확인
cat /etc/hosts | grep cubrid

# 포트별 접속 테스트
curl -I http://ha-ms-0.default.cms.cubrid.com
curl -I https://ha-ms-0.default.cms.cubrid.com
curl -I https://ha-ms-0.default.cms.cubrid.com:30443

# IP 직접 접속 테스트 (DNS 문제 우회)
curl -I http://192.168.1.100:30443
```

### 3. CMS 서비스에 연결할 수 없는 경우

```bash
# CMS 서비스 상태 확인
kubectl get svc | grep cms-svc

# 서비스 엔드포인트 확인
kubectl get endpoints | grep cms-svc

# 파드 상태 확인
kubectl get pods -l app=cubrid-operator-webhook-dep
```

### 4. 포트 충돌 문제

```bash
# 포트 사용 현황 확인
kubectl get svc -o wide | grep 8001

# 포트 충돌 해결을 위해 다른 포트 사용
```


### 배포 및 확인

```bash
# CubridDB CR 배포
kubectl apply -f production-cubrid.yaml

# 자동 생성된 Ingress 확인
kubectl get ingress production-cubrid-production-cms-ingress

# Ingress 상세 정보 확인
kubectl describe ingress production-cubrid-production-cms-ingress

# 생성된 호스트 확인
kubectl get ingress production-cubrid-production-cms-ingress -o jsonpath='{.spec.rules[*].host}'
```

### 생성되는 리소스

Operator가 자동으로 생성하는 리소스들:

1. **Ingress**: `production-cubrid-production-cms-ingress`
2. **CMS 서비스들**: 
   - `ha-ms-0-production-cms-svc`
   - `ha-ms-1-production-cms-svc`
   - `ha-ms-2-production-cms-svc`
3. **호스트들**:
   - `ha-ms-0.production.cms.cubrid.com`
   - `ha-ms-1.production.cms.cubrid.com`
   - `ha-ms-2.production.cms.cubrid.com`

이 가이드를 따라 Ingress를 통해 CUBRID CMS 서비스에 안전하고 효율적으로 접근할 수 있습니다.

## 요약

### 서비스 구성 방식

| 구성 방식 | HA Mode | 단일 서버 |
|-----------|---------|-----------|
| **CMS 접속 방식** | Ingress + Ingress CMS Service | NodePort Service |
| **서비스 타입** | ClusterIP (Headless) | NodePort |
| **접속 방법** | 도메인 이름 (포트 포함) | 노드IP:NodePort |
| **예시** | `ha-ms-0.default.cms.cubrid.com:30443` | `192.168.1.100:31000` |

### 주요 특징

- **HA Mode**: Ingress를 통한 도메인 기반 접속으로 고가용성과 확장성 제공
- **단일 서버**: NodePort를 통한 직접 접속으로 간단한 구성 제공
- **자동 생성**: Operator가 구성에 따라 적절한 서비스를 자동으로 생성
- **CR 기반 관리**: 모든 서비스는 Custom Resource를 통해 관리됨

### 권장 사항

- **HA Mode**: 프로덕션 환경에서 고가용성이 필요한 경우
- **단일 서버**: 개발/테스트 환경이나 단순한 구성이 필요한 경우
- **보안**: 프로덕션 환경에서는 항상 HTTPS와 적절한 인증 메커니즘 사용 

