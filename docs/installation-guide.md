# 설치 가이드

cubrid-operator를 설치하는 방법은 두 가지가 있습니다:

## 설치 방법

### 1. Helm을 사용한 설치
Helm 차트를 사용하여 cubrid-operator를 설치하려면 [Helm 차트 가이드](./helm-chart-guide.md)를 참조하세요.

### 2. 매니페스트를 사용한 설치
Kubernetes 매니페스트 파일을 직접 적용하여 cubrid-operator를 설치하려면 [매니페스트 설치 가이드](./manifest-installation-guide.md)를 참조하세요.

## 권장사항

- **개발/테스트 환경**: 매니페스트 설치 방법을 권장합니다.
- **프로덕션 환경**: Helm 차트 설치 방법을 권장합니다.

## 사전 요구사항

- Kubernetes 클러스터 (v1.19 이상)
- kubectl이 설치되어 있어야 함
- Helm 설치 (Helm 차트 사용 시)

---

자세한 설치 방법과 설정 옵션은 위의 각 가이드 문서를 참조하세요. 