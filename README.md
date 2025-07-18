# CUBRID Operator

CUBRID Operator는 Kubernetes에서 CUBRID 데이터베이스를 관리하기 위한 operator입니다.

## 주요 기능

- **자동화된 배포**: CUBRID 데이터베이스 인스턴스의 자동 배포 및 구성
- **고가용성**: Master-Slave-Replica 구조를 통한 고가용성 제공
- **백업 관리**: 자동화된 백업 스케줄링 및 관리
- **스케일링**: 수평 및 수직 스케일링 지원
- **모니터링**: 상태 모니터링

## 빠른 시작

### 1. Operator 설치

#### Helm을 사용한 설치 (권장)

```bash
helm repo add cubrid https://cubrid.github.io/helm-charts
helm repo update
helm install cubrid-operator-crds cubrid/cubrid-operator-crds
helm install cubrid-operator cubrid/cubrid-operator
```

#### Manifest 파일을 사용한 설치

```bash
kubectl apply -f https://raw.githubusercontent.com/CUBRID/cubrid-operator/develop/deploy/manifests/cubrid-operator.yaml
```

### 2. CUBRID 인스턴스 생성

```bash
kubectl apply -f - <<EOF
apiVersion: k8s.cubrid.com/v1
kind: CubridDB
metadata:
  name: my-cubrid-db
spec:
  image: cubrid/cubrid
  replication:
    enable: false
  broker:
  - name: broker-svc
    port: 30000
    servicePort: 30100
    serviceType: NodePort
EOF
```

자세한 설치 방법은 [설치 가이드](./docs/installation-guide.md)를 참조하세요.

## 개발 환경 설정

이 프로젝트는 일관된 개발 환경을 위해 필요한 Go 도구들은 `./bin` 폴더에 download 됩니다.

### Go tools

- kustomize (v5.3.0) - Kubernetes 매니페스트 관리
- controller-gen (v0.14.0) - Kubernetes 컨트롤러 코드 생성
- envtest (release-0.17) - 테스트 환경 설정
- golangci-lint (v1.54.2) - Go 코드 린터
- gofumpt (v0.8.0) - Go 코드 포맷터

자세한 개발 환경 설정은 [개발자 가이드](./docs/developer-guide.md)를 참조하세요.

## CI/CD

이 프로젝트는 GitHub Actions를 통해 자동화된 CI/CD 파이프라인을 제공합니다.

- **Lint 검사**: PR 생성 시 자동으로 golangci-lint와 gofumpt 검사 실행
- **빌드 검증**: main 브랜치에서 빌드 테스트 실행

자세한 내용은 [CI/CD 가이드](./docs/ci-cd-guide.md)를 참조하세요.


## 문서

- [설치 가이드](./docs/installation-guide.md) - 상세한 설치 방법
- [API Reference](./docs/api-reference.md) - CRD API 문서
- [개발자 가이드](./docs/developer-guide.md) - 개발 환경 설정 및 기여 방법
- [Helm Chart 가이드](./docs/helm-chart-guide.md) - Helm Chart 사용법
- [스토리지 프로비저닝 가이드](./docs/storage-provisioning.md) - 스토리지 설정
- [서비스 관리 가이드](./docs/service-management.md) - 서비스 구성
- [CMS 접속 가이드](./docs/cms-access-guide.md) - CMS 서비스 접속 방법
- [Broker Endpoint 가이드](./docs/broker-endpoint-controller.md) - 브로커 엔드포인트 관리


## 프로젝트 정보

- **소스 코드**: [https://github.com/CUBRID/cubrid-operator](https://github.com/CUBRID/cubrid-operator)
- **Helm Charts**: [https://github.com/CUBRID/helm-charts](https://github.com/CUBRID/helm-charts)

## 라이선스

이 프로젝트는 [Apache License 2.0](LICENSE) 하에 배포됩니다.