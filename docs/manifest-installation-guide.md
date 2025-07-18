# 매니페스트 설치 가이드

이 가이드는 Kubernetes 매니페스트를 사용하여 CUBRID Operator를 설치하는 방법을 상세히 설명합니다.

## 사전 요구 사항

- Kubernetes 1.29+
- kubectl 1.11.3+

## 설치 방법

### 1. 직접 설치

GitHub에서 제공하는 매니페스트를 직접 적용하여 설치할 수 있습니다:

```bash
kubectl apply -f https://raw.githubusercontent.com/CUBRID/cubrid-operator/v1.0/deploy/manifests/cubrid-operator.yaml
```

### 2. 로컬 파일로 설치

1. 매니페스트 파일 다운로드:
```bash
curl -O https://raw.githubusercontent.com/CUBRID/cubrid-operator/v1.0/deploy/manifests/cubrid-operator.yaml
```

2. 설치:
```bash
kubectl apply -f cubrid-operator.yaml
```

### 3. 설치 확인

```bash
# CRDs 확인
kubectl get crd | grep cubrid

# Operator 파드 확인
kubectl get pods -n cubrid

# Operator 서비스 확인
kubectl get services -n cubrid
```

## 제거

```bash
# GitHub 매니페스트로 제거
kubectl delete -f https://raw.githubusercontent.com/CUBRID/cubrid-operator/v1.0/deploy/manifests/cubrid-operator.yaml

# 또는 로컬 파일로 제거
kubectl delete -f cubrid-operator.yaml
```

## 주의 사항

1. 매니페스트로 설치 시 기본적으로 내부 cert-manager가 사용됩니다.
2. 외부 cert-manager를 사용하려면 Helm 차트 설치를 권장합니다.
3. 제거 시 모든 CUBRID 관련 리소스(CRDs, Deployments, Services 등)가 함께 삭제됩니다.

## 문제 해결

설치 중 문제가 발생하면 다음을 확인하세요:

1. 네임스페이스 상태:
```bash
kubectl get namespace cubrid
```

2. 파드 로그:
```bash
kubectl logs -n cubrid deployments/cubrid-operator-dep
kubectl logs -n cubrid deployments/cubrid-operator-webhook-dep
```

3. 이벤트:
```bash
kubectl get events -n cubrid
``` 