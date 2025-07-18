# 개발자 가이드

이 문서는 cubrid-operator 프로젝트 개발을 위한 환경 설정과 도구 사용법을 설명합니다.

## 1. 개발 도구 설치

### 사전 요구 사항

- Go 1.23.0+
- Docker 26.1.4+
- kubectl 1.30.1+
- Helm 3.0+

### Go 설치
cubrid-operator는 Go 1.23.0 이상이 필요합니다.

```bash
# Linux/macOS
wget https://go.dev/dl/go1.23.11.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.11.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### kubectl 설치
Kubernetes 클러스터와 상호작용하기 위해 kubectl 1.30.1이 필요합니다.

```bash
# Linux (CentOS/Rocky Linux)
curl -LO "https://dl.k8s.io/release/v1.30.1/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
```

### Helm 설치
Kubernetes 패키지 매니저인 Helm이 필요합니다.

```bash
# Linux/macOS
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

### Docker 설치 (선택사항)
로컬에서 컨테이너 이미지를 빌드하려면 Docker가 필요합니다.

```bash
# CentOS/Rocky Linux
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker $USER

```

## 2. 개발 환경 설정

### 프로젝트 클론
```bash
git clone https://github.com/CUBRID/cubrid-operator.git
cd cubrid-operator
```

### 의존성 설치 (선택사항)
go mod download

### Go tools 설치
프로젝트에서 사용하는 Go tools들을 설치합니다.

```bash
# 프로젝트 내 bin 디렉토리에 Go tools 설치
make install-tools
```


**설치되는 Go tools:**
- kustomize (v5.3.0) - Kubernetes 매니페스트 관리
- controller-gen (v0.14.0) - Kubernetes 컨트롤러 코드 생성
- envtest (release-0.17) - 테스트 환경 설정
- golangci-lint (v1.54.2) - Go 코드 린터
- gofumpt (v0.8.0) - Go 코드 포맷터

### kubectl 클러스터 관리자 권한 설정
Kubernetes 클러스터에 클러스터 관리자 권한으로 접근하기 위한 설정입니다.

```bash
# 1. kubeconfig 디렉토리 생성
mkdir -p $HOME/.kube

# 2. 관리자 인증서 복사 (API 서버 노드에서 실행)
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config

# 3. 소유권 변경 (일반 사용자 권한으로 kubectl 사용 가능)
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 4. 연결 테스트
kubectl cluster-info
kubectl get nodes

# 5. 권한 확인
kubectl auth can-i --list
```

### 환경 변수 설정 (선택사항)
```bash
# Go 모듈 설정 (기본값이 잘 동작하므로 필요시에만 설정)
# export GO111MODULE=on
# export GOPROXY=direct

# kubectl 컨텍스트 설정 (필요시)
kubectl config use-context <your-cluster-context>
```

## 3. Makefile 명령어 가이드

프로젝트의 Makefile을 통해 다양한 개발 작업을 수행할 수 있습니다. 자세한 내용은 프로젝트 루트의 `Makefile`을 참조하세요.

### 기본 명령어

#### 빌드
```bash
# 전체 프로젝트 빌드
make build
```

#### 코드 품질 검사
```bash
# golangci-lint 실행
make lint

# 코드 포맷팅
make fmt
```

#### 정리
```bash
# 빌드 결과물 및 캐시 정리
make clean
```

### Go tools 관리

#### Go tools 설치
```bash
# 개발에 필요한 모든 Go tools 설치
make install-tools
```

#### Go tools 버전 확인
```bash
# 설치된 Go tools들의 버전 확인
make version
```

### 컨테이너 관련

#### Docker 이미지 빌드
```bash
# 로컬에서 Docker 이미지 빌드
make docker-build

# 특정 버전 태그로 빌드
make docker-build cubrid/operator:latest

# 또는 IMG 변수 사용
make docker-build IMG=cubrid/operator:v1.0.0
```

**Docker 이미지 태그 포맷:**
```
[registry/]image-name:tag
```

### 배포 관련

#### 매니페스트 및 CRDs 생성
```bash
# RBAC, Webhook, CRDs 설정 생성
make manifests
```

#### 설치/제거
```bash
# build-installer로 생성한 YAML 파일로 설치
make build-installer
kubectl apply -f deploy/manifests/cubrid-operator.yaml

# build-installer로 생성한 YAML 파일로 제거
kubectl delete -f deploy/manifests/cubrid-operator.yaml

# Helm 차트 설치
make helm-install

# Helm 차트 제거
make helm-uninstall
```

