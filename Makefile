
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.0
# settings
REPO_ROOT:=${CURDIR}
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GOOS?=$(shell hack/tool/goos.sh)
GOARCH?=$(shell hack/tool/goarch.sh)

REG ?=registry.cn-hangzhou.aliyuncs.com/aoxn
IMG ?=$(REG)/meridian
TAG?=$(shell hack/tool/tag.sh)

# the output binary name, overridden when cross compiling
KIND_BINARY_NAME?=meridian
# use the official module proxy by default
GOPROXY?=https://goproxy.cn,direct
# default build image
GO_VERSION?=1.22
GO_IMAGE?=$(REG)/golang:$(GO_VERSION)
# docker volume name, used as a go module / build cache
CACHE_VOLUME?=meridian-build-cache

# variables for consistent logic, don't override these
CONTAINER_REPO_DIR=/src/meridian
CONTAINER_OUT_DIR=$(CONTAINER_REPO_DIR)/bin
OUT_DIR=$(REPO_ROOT)/bin


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

# creates the cache volume
make-cache:
	@echo + Ensuring build cache volume exists
	sudo docker volume create $(CACHE_VOLUME) || true

# cleans the cache volume
clean-cache:
	@echo + Removing build cache volume
	sudo docker volume rm $(CACHE_VOLUME)

# creates the output directory
out-dir:
	@echo + Ensuring build output directory exists
	mkdir -p $(OUT_DIR)

# cleans the output directory
clean-output:
	@echo + Removing build output directory
	rm -rf $(OUT_DIR)/

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

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.54.2
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: manager
manager: fmt vet
	@echo Build manager binary.
	go build -o bin/manager -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" cmd/manager/main.go

.PHONY: mdx86
mdx86: fmt vet
	@echo Build meridian binary.
	GOOS=darwin		  \
	GOARCH=amd64              \
	CGO_ENABLED=1 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" -o bin/meridian.darwin.x86_64 cmd/main.go
	codesign --entitlements vz.entitlements -s - bin/meridian.darwin.x86_64 || true
	sudo cp -rf bin/meridian.darwin.x86_64 /usr/local/bin/meridian

.PHONY: mlx86
mlx86: fmt vet
	@echo Build meridian binary.
	GOOS=linux                \
	GOARCH=amd64              \
	CGO_ENABLED=0 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" -o bin/meridian.linux.x86_64 cmd/main.go
	codesign --entitlements vz.entitlements -s - bin/meridian.linux.x86_64 || true

.PHONY: meridian-guest
meridian-guest: fmt vet
	@echo Build meridian-guest binary[$(GOOS)][$(GOARCH)].
	GOOS=$(GOOS)                \
	GOARCH=$(GOARCH)             \
	CGO_ENABLED=0 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" -o bin/meridian-guest.$(GOOS).$(GOARCH) cmd/meridian-guest/guest.go
	sudo cp -rf bin/meridian-guest.$(GOOS).$(GOARCH) /usr/local/bin/meridian-guest

.PHONY: meridian-node
meridian-node: fmt vet
	@echo Build meridian-node binary[$(GOOS)][$(GOARCH)].
	GOOS=$(GOOS)                \
	GOARCH=$(GOARCH)             \
	CGO_ENABLED=0 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" -o bin/meridian-node.$(GOOS).$(GOARCH) cmd/meridian-node/node.go
	sudo cp -rf bin/meridian-node.$(GOOS).$(GOARCH) /usr/local/bin/meridian-node

.PHONY: meridian
meridian: fmt vet
	@echo Build meridian binary[$(GOOS)][$(GOARCH)].
	GOOS=$(GOOS)                \
	GOARCH=$(GOARCH)             \
	CGO_ENABLED=1 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" -o bin/meridian.$(GOOS).$(GOARCH) cmd/meridian/main.go
	sudo cp -rf bin/meridian.$(GOOS).$(GOARCH) /usr/local/bin/meridian

.PHONY: meridiand
meridiand: fmt vet
	@echo Build meridiand binary[$(GOOS)][$(GOARCH)].
	GOOS=$(GOOS)                \
	GOARCH=$(GOARCH)             \
	CGO_ENABLED=1 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" -o bin/meridiand.$(GOOS).$(GOARCH) cmd/meridiand/daemon.go
	sudo cp -rf bin/meridiand.$(GOOS).$(GOARCH) /usr/local/bin/meridiand


.PHONYE: universal
universal: fmt vet
	sudo rm -rf bin/meridian.darwin.* /usr/local/bin/meridian /usr/local/bin/meridiand bin/meridian bin/meridiand
	@echo Build meridian binary[amd64].
	GOOS=darwin 		  \
	GOARCH=amd64              \
	CGO_ENABLED=1 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" -o bin/meridiand.darwin.x86_64 cmd/meridiand/daemon.go
	codesign --entitlements vz.entitlements -s - bin/meridiand.darwin.x86_64 || true
	#-ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w -extldflags '-static'"

	@echo Build meridian binary[aarch64].
	GOOS=darwin              \
	GOARCH=arm64             \
	CGO_ENABLED=1 go build -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w " -o bin/meridiand.darwin.aarch64 cmd/meridiand/daemon.go
	codesign --entitlements vz.entitlements -s - bin/meridiand.darwin.aarch64 || true

	@echo create universal arch app bin/meridiand
	lipo -create -output bin/meridiand bin/meridiand.darwin.aarch64 bin/meridiand.darwin.x86_64
	sudo cp -rf bin/meridiand /usr/local/bin/meridiand


.PHONY: amd
amd: 
	@echo Build meridiand amd64 binary.
	GOOS=linux                \
	GOARCH=amd64              \
	GOPROXY=${GOPROXY}        \
	go build                  \
	    -o bin/meridian.amd64 \
	    -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" \
	    cmd/main.go

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: docker
docker: docker-build docker-push

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG}:${TAG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}:${TAG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your generic (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG}:${TAG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

# builds meridian in a container, outputs to $(OUT_DIR)
container: make-cache out-dir
	@echo + Building binary:$(OUT_DIR)/$(KIND_BINARY_NAME)
	sudo docker run \
		--rm \
		-v $(CACHE_VOLUME):/go \
		-e GOCACHE=/go/cache \
		-v $(OUT_DIR):/out \
		-v $(REPO_ROOT):$(CONTAINER_REPO_DIR) \
		-w $(CONTAINER_REPO_DIR) \
		-e GO111MODULE=on \
		-e GOPROXY=$(GOPROXY) \
		-e CGO_ENABLED=0 \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-e HTTP_PROXY=$(HTTP_PROXY) \
		-e HTTPS_PROXY=$(HTTPS_PROXY) \
		-e NO_PROXY=$(NO_PROXY) \
		--user $(UID):$(GID) \
		$(GO_IMAGE) \
		go build -v -o /out/$(KIND_BINARY_NAME) \
		    -ldflags "-X github.com/aoxn/meridian.Version=$(TAG) -s -w" cmd/main.go


##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: 
	ossutil cp bin/meridian.x86_64 oss://host-wdrip-cn-hangzhou/bin/${GOOS}/${GOARCH}/${VERSION}/meridian

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies
.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v5.2.1
CONTROLLER_TOOLS_VERSION ?= v0.14.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
