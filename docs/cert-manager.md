# CUBRID Operator Webhook Certificate Management Guide

## 개요

CUBRID Operator의 webhook 서버에서 사용하는 인증서 관리 방식에 대해 설명합니다.

## 개요

CUBRID Operator의 webhook 서버는 두 가지 방식으로 TLS 인증서를 관리할 수 있습니다:

1. 내부 cert-manager (기본값)
   - Operator가 자체적으로 인증서를 생성하고 관리
   - 별도의 cert-manager 설치가 필요 없음

2. 외부 cert-manager
   - Kubernetes cert-manager를 사용하여 인증서 관리
   - 자동 인증서 갱신 등 cert-manager의 기능 활용 가능

## 사전 요구 사항

외부 cert-manager를 사용하는 경우, 다음 사항이 필요합니다:

1. cert-manager v1.13.3 이상이 클러스터에 설치되어 있어야 합니다.
   ```bash
   # cert-manager 설치
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.3/cert-manager.yaml
   ```

2. cert-manager가 정상적으로 동작하는지 확인:
   ```bash
   kubectl get pods -n cert-manager
   NAME                                       READY   STATUS    RESTARTS   AGE
   cert-manager-6576cfbf76-9pn96              1/1     Running   0          3d2h
   cert-manager-cainjector-6fd5bb64d4-vxmgb   1/1     Running   0          3d2h
   cert-manager-webhook-7f765545b6-6vtn5      1/1     Running   0          3d2h
   ```

## 배포 방법

### 1. Helm 차트를 사용한 배포

Helm 차트를 사용하여 배포할 때는 `values.yaml`의 `certManagerType` 값을 통해 인증서 관리 방식을 선택할 수 있습니다.

1. 내부 cert-manager 사용 (기본값):
   ```bash
   helm install cubrid-operator ./deploy/charts/cubrid-operator
   ```
   또는
   ```bash
   helm install cubrid-operator ./deploy/charts/cubrid-operator --set webhook.certManagerType=internal
   ```

2. 외부 cert-manager 사용:
   ```bash
   helm install cubrid-operator ./deploy/charts/cubrid-operator --set webhook.certManagerType=external
   ```

### 2. Make 명령어를 사용한 배포

`make deploy` 명령어를 사용할 때 `CERT_MANAGER_TYPE` 환경변수를 통해 인증서 관리 방식을 선택할 수 있습니다.

1. 내부 cert-manager 사용 (기본값):
   ```bash
   make deploy IMG=cubrid/cubrid-operator:latest
   ```
   또는
   ```bash
   make deploy IMG=cubrid/cubrid-operator:latest CERT_MANAGER_TYPE=internal
   ```

2. 외부 cert-manager 사용:
   ```bash
   make deploy IMG=cubrid/cubrid-operator:latest CERT_MANAGER_TYPE=external
   ```

## 동작 방식

### 내부 cert-manager

- Operator가 직접 인증서와 개인 키를 생성
- `/tmp/k8s-webhook-server/serving-certs` 디렉토리에 인증서 파일 저장
- 인증서 갱신은 Operator가 직접 처리

### 외부 cert-manager

- cert-manager가 Certificate/Issuer 리소스를 통해 인증서 생성
- 생성된 인증서는 Secret에 저장
- Secret이 webhook 서버의 `/tmp/k8s-webhook-server/serving-certs` 디렉토리에 마운트
- cert-manager가 인증서 갱신을 자동으로 처리 (만료 30일 전)

## 인증서 설정

외부 cert-manager 사용 시 생성되는 리소스:

1. Issuer (selfsigned)
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: cubrid-operator-webhook-selfsigned-issuer
     namespace: cubrid
   spec:
     selfSigned: {}
   ```

2. Certificate
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: cubrid-operator-webhook-serving-cert
     namespace: cubrid
   spec:
     dnsNames:
     - cubrid-operator-webhook-service.cubrid.svc
     - cubrid-operator-webhook-service.cubrid.svc.cluster.local
     issuerRef:
       kind: Issuer
       name: cubrid-operator-webhook-selfsigned-issuer
     secretName: cubrid-operator-webhook-cert
     duration: 8760h0m0s # 1년
     renewBefore: 720h0m0s # 30일
   ```

