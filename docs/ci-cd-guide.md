# CI/CD 가이드

이 문서는 cubrid-operator 프로젝트의 CI/CD 파이프라인 구조와 동작 방식을 설명합니다.

## 전체 구조

- GitHub Actions를 사용하여 코드 품질 검사, 빌드, 테스트, Docker 이미지 빌드 및 배포를 자동화합니다.
- 주요 워크플로우는 다음과 같습니다:
  - PR(Pull Request) 생성/업데이트 시: Lint, Build 자동 실행
  - main 브랜치 머지 시: Lint, Build 자동 실행
  
## 워크플로우 상세

### 1. Lint
- 트리거: PR에 커밋이 들어올 때마다
- 동작: golangci-lint로 코드 스타일 및 정적 분석 수행
- 위치: `.github/workflows/lint.yml`
- 상세 설정: `.golangci.yml` 파일을 참조

### 2. Build
- 트리거: PR에 커밋이 들어올 때마다
- 동작: go build로 전체 프로젝트 빌드
- 위치: `.github/workflows/build.yml`


## 참고
- 워크플로우 파일들은 `.github/workflows/` 디렉토리에 위치합니다.
- 상세 설정 및 예시는 각 워크플로우 파일을 참고하세요.

---
