#
#  Copyright 2016 CUBRID Corporation
# 
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
# 
#       http://www.apache.org/licenses/LICENSE-2.0
# 
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
# 
#


# Image URL to use all building/pushing image targets
IMG ?= cubrid/operator:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0

# Certificate manager type (internal or external)
CERT_MANAGER_TYPE ?= internal

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


##@ Helm Chart
# Default namespace for deployment
NAMESPACE := cubrid

CRD_DIR := deploy/charts/cubrid-operator-crds/templates
COMBINED_CRD_FILE := $(CRD_DIR)/crds.yaml

# CRDs Path
CRD_SOURCES := config/crd/bases/k8s.cubrid.com_cubriddbs.yaml config/crd/bases/k8s.cubrid.com_backupdbs.yaml

# Helm Chart Directory and Package Configuration
CHARTS_DIR := deploy/charts
HELM_REPO_DIR := helm-repo
HELM_REPO_URL := https://airnet73.github.io/test-operator
CHARTS := cubrid-operator cubrid-operator-crds

# Helm Package Directory Creation
$(HELM_REPO_DIR):
	@mkdir -p $(HELM_REPO_DIR)

# Get version from Chart.yaml
VERSION := $(shell grep '^version:' $(CHARTS_DIR)/cubrid-operator/Chart.yaml | cut -d' ' -f2)

# helm-crds 
.PHONY: helm-crd
helm-crd: manifests ## Generate crd file for Helm Charts
	@echo "Creating CRD directory if it doesn't exist..."
	@mkdir -p $(CRD_DIR)
	@echo "Combining CRD files into $(COMBINED_CRD_FILE)..."
	@> $(COMBINED_CRD_FILE) 
	@cat $(CRD_SOURCES) >> $(COMBINED_CRD_FILE) 
	@echo "CRDs combined successfully into $(COMBINED_CRD_FILE)."


.PHONY: helm-package 
helm-package: $(CHARTS) ## Package All Charts (cubrid-operator, cubrid-operator-crds)
$(CHARTS):
	@echo "Packaging $@ chart..."
	helm package $(CHARTS_DIR)/$@ --destination $(HELM_REPO_DIR)
	@echo "$@ chart packaged successfully."

.PHONY: helm-update-repo
helm-update-repo: helm-package ## Create or Update Helm Repository Index
	@echo "Updating Helm repository index..."
	helm repo index $(HELM_REPO_DIR) --url $(HELM_REPO_URL)
	@echo "Helm repository index updated successfully."

.PHONY: helm-install
helm-install: ## Install Helm Charts (cubrid-operator, cubrid-operator-crds)
	@echo "Installing $(word 2,$(CHARTS)) chart..."
	helm install $(word 2,$(CHARTS)) $(HELM_REPO_DIR)/$(word 2,$(CHARTS))-$(VERSION).tgz --namespace $(NAMESPACE) --create-namespace	
	@echo "Installing $(word 1,$(CHARTS)) chart..."
	helm install $(word 1,$(CHARTS)) $(HELM_REPO_DIR)/$(word 1,$(CHARTS))-$(VERSION).tgz --namespace $(NAMESPACE) --create-namespace
	@echo "Helm charts installed successfully."

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall Helm Charts (cubrid-operator, cubrid-operator-crds)
	@echo "Uninstalling $(word 1,$(CHARTS)) chart..."
	helm uninstall $(word 1,$(CHARTS)) --namespace $(NAMESPACE)
	@echo "Uninstalling $(word 2,$(CHARTS)) chart..."
	helm uninstall $(word 2,$(CHARTS)) --namespace $(NAMESPACE)
	@echo "Helm charts uninstalled successfully."

.PHONY: helm-template
helm-template: ## Render helm chart templates without installation
	helm template $(word 1,$(CHARTS)) $(CHARTS_DIR)/$(word 1,$(CHARTS))

.PHONY: helm-template-values
helm-template-values: ## Render helm chart templates with values file. Usage: make helm-template-values VALUES_FILE=path/to/values.yaml
	helm template $(word 1,$(CHARTS)) $(CHARTS_DIR)/$(word 1,$(CHARTS)) -f $(if $(VALUES_FILE),$(VALUES_FILE),$(CHARTS_DIR)/$(word 1,$(CHARTS))/values.yaml)

.PHONY: helm-template-debug
helm-template-debug: ## Render helm chart templates in debug mode
	helm template $(word 1,$(CHARTS)) $(CHARTS_DIR)/$(word 1,$(CHARTS)) --debug

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=cubrid-operator-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: gofumpt ## Run gofumpt against code.
	$(GOFUMPT) -w .
	

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

#.PHONY: test
#test: manifests generate fmt vet envtest ## Run tests.
#	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
#.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
#test-e2e:
#	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet gofumpt ## Build manager binary.
	go build -o bin/manager cmd/main.go


.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run -gcflags "all=-N -l" ./cmd/main.go

.PHONY: run-webhook
run-webhook: manifests generate fmt vet ## Run a webhook server from your host.
	go run -gcflags "all=-N -l" ./cmd/main.go webhook

.PHONY: docker-build
docker-build: ## Build docker image with the manager. Usage: make docker-build [IMG=myregistry/image:tag]
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-images
docker-images: ## List all docker images
	@echo "=== Docker Images ==="
	@$(CONTAINER_TOOL) images

.PHONY: docker-push
docker-push: ## Push docker image with the manager. Usage: make docker-push [IMG=myregistry/image:tag]
	$(CONTAINER_TOOL) push ${IMG}

# Note: This project supports Linux platform only.
# For cross-platform support, consider using docker buildx manually.

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment for users (fixed to cubrid namespace)
	mkdir -p deploy/manifests
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
ifeq ($(CERT_MANAGER_TYPE),external)
	cp config/default/kustomization.yaml config/default/kustomization.yaml.bak
	sed -i 's/#- ..\/certmanager/- ..\/certmanager/' config/default/kustomization.yaml
	sed -i 's/#- webhookcainjection_patch.yaml/- webhookcainjection_patch.yaml/' config/default/kustomization.yaml
	sed -i 's/#replacements:/replacements:/' config/default/kustomization.yaml
	cp config/default/kustomization.yaml config/default/kustomization.yaml.debug
	$(KUSTOMIZE) build config/default | sed 's/\$$(CERT_MANAGER_TYPE)/external/g' > deploy/manifests/cubrid-operator.yaml
	mv config/default/kustomization.yaml.bak config/default/kustomization.yaml
else
	$(KUSTOMIZE) build config/default | sed 's/\$$(CERT_MANAGER_TYPE)/internal/g' > deploy/manifests/cubrid-operator.yaml
endif

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install-crd
install-crd: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall-crd
uninstall-crd: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config. Usage: make deploy NAMESPACE=my-namespace
	@echo "Deploying CUBRID Operator to namespace: $(NAMESPACE)"
	@$(KUBECTL) create namespace $(NAMESPACE) --dry-run=client -o yaml | $(KUBECTL) apply -f -
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}

ifeq ($(CERT_MANAGER_TYPE),external)
	cp config/default/kustomization.yaml config/default/kustomization.yaml.bak
	sed -i 's/#- ..\/certmanager/- ..\/certmanager/' config/default/kustomization.yaml
	sed -i 's/#- webhookcainjection_patch.yaml/- webhookcainjection_patch.yaml/' config/default/kustomization.yaml
	sed -i 's/#replacements:/replacements:/' config/default/kustomization.yaml
	cp config/default/kustomization.yaml config/default/kustomization.yaml.debug
	$(KUSTOMIZE) build config/default | sed 's/\$$(CERT_MANAGER_TYPE)/external/g' | sed 's/namespace: cubrid/namespace: $(NAMESPACE)/g' | sed 's/--webhook-namespace=cubrid/--webhook-namespace=$(NAMESPACE)/g' | sed 's/^  name: cubrid$$/  name: $(NAMESPACE)/g' | $(KUBECTL) apply -f -
	mv config/default/kustomization.yaml.bak config/default/kustomization.yaml
	@echo "Debug version of kustomization.yaml saved as config/default/kustomization.yaml.debug"
else
	$(KUSTOMIZE) build config/default | sed 's/\$$(CERT_MANAGER_TYPE)/internal/g' | sed 's/namespace: cubrid/namespace: $(NAMESPACE)/g' | sed 's/--webhook-namespace=cubrid/--webhook-namespace=$(NAMESPACE)/g' | sed 's/^  name: cubrid$$/  name: $(NAMESPACE)/g' | $(KUBECTL) apply -f -
endif

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Usage: make undeploy NAMESPACE=my-namespace. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@echo "Undeploying CUBRID Operator from namespace: $(NAMESPACE)"
	$(KUSTOMIZE) build config/default | sed 's/namespace: cubrid/namespace: $(NAMESPACE)/g' | sed 's/--webhook-namespace=cubrid/--webhook-namespace=$(NAMESPACE)/g' | sed 's/^  name: cubrid$$/  name: $(NAMESPACE)/g' | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -


##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
GOFUMPT ?= $(LOCALBIN)/gofumpt-$(GOFUMPT_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= release-0.17
GOLANGCI_LINT_VERSION ?= v1.60.0
GOFUMPT_VERSION ?= v0.8.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# .PHONY: goimports
# goimports: $(GOIMPORTS) ## Download goimports locally if necessary.
# $(GOIMPORTS): $(LOCALBIN)
# 	$(call go-install-tool,$(GOIMPORTS),golang.org/x/tools/cmd/goimports,latest)

.PHONY: gofumpt
gofumpt: $(GOFUMPT) ## Download gofumpt locally if necessary.
$(GOFUMPT): $(LOCALBIN)
	@[ -f $(GOFUMPT) ] || { \
		set -e; \
		echo "Downloading mvdan.cc/gofumpt@latest" ;\
		GOBIN=$(LOCALBIN) go install mvdan.cc/gofumpt@latest ;\
		mv $(LOCALBIN)/gofumpt $(GOFUMPT) ;\
	}

.PHONY: install-tools
install-tools: kustomize controller-gen envtest golangci-lint gofumpt ## Install all development tools
	@echo "All development tools have been installed successfully!"

.PHONY: clean
clean: ## Clean build artifacts and cache
	rm -rf bin/
	go clean -cache

.PHONY: version
version: ## Show version information
	@echo "=== Version Information ==="
	@echo "Project Version: $(VERSION)"
	@echo "Go Version: $(shell go version)"
	@echo "Kustomize: $(KUSTOMIZE_VERSION)"
	@echo "Controller-gen: $(CONTROLLER_TOOLS_VERSION)"
	@echo "Golangci-lint: $(GOLANGCI_LINT_VERSION)"
	@echo "Gofumpt: $(GOFUMPT_VERSION)"

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

