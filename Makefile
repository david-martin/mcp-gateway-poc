# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec
MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))
PROJECT_NAMESPACE ?= mcp-system

OS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m | tr '[:upper:]' '[:lower:]')
# Container Engine to be used for building image and with kind
CONTAINER_ENGINE ?= docker
ifeq (podman,$(CONTAINER_ENGINE))
	CONTAINER_ENGINE_EXTRA_FLAGS ?= --load
endif

# go-install-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

KUSTOMIZE = $(PROJECT_PATH)/bin/kustomize
$(KUSTOMIZE):
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.5.5)

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.	

YQ=$(PROJECT_PATH)/bin/yq
YQ_VERSION := v4.34.2
$(YQ):
	$(call go-install-tool,$(YQ),github.com/mikefarah/yq/v4@$(YQ_VERSION))

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.

.PHONY: namespace
namespace: ## Creates a namespace where to deploy Kuadrant Operator
	kubectl create namespace $(PROJECT_NAMESPACE)


.PHONY: k8s-env-setup
k8s-env-setup: ## Install Kuadrant CRDs and dependencies.
	$(MAKE) install-metallb
	$(MAKE) istioctl-install
	$(MAKE) gateway-api-install
	$(MAKE) gateway-install
	$(MAKE) mcp-server-install


.PHONY: local-env-setup
local-env-setup: ## k8s-env-setup based on Kind cluster
	$(MAKE) kind-delete-cluster
	$(MAKE) kind-create-cluster
	$(MAKE) k8s-env-setup

deploy-dependencies: kustomize ## Deploy dependencies to the K8s cluster specified in ~/.kube/config.
	$(MAKE) namespace
	$(KUSTOMIZE) build config/dependencies | kubectl apply --server-side -f -
	kubectl -n "$(PROJECT_NAMESPACE)" wait --timeout=300s --for=condition=Available deployments --all

.PHONY: install-metallb
install-metallb: SUBNET_OFFSET=1
install-metallb: CIDR=28
install-metallb: NUM_IPS=16
install-metallb: kustomize yq ## Installs the metallb load balancer allowing use of an LoadBalancer type with a gateway
	$(KUSTOMIZE) build config/metallb | kubectl apply -f -
	kubectl -n metallb-system wait --for=condition=Available deployments controller --timeout=300s
	kubectl -n metallb-system wait --for=condition=ready pod --selector=app=metallb --timeout=120s
	./utils/docker-network-ipaddresspool.sh kind $(YQ) ${SUBNET_OFFSET} ${CIDR} ${NUM_IPS} | kubectl apply -n metallb-system -f -	
# Include last to avoid changing MAKEFILE_LIST used above
include ./make/*.mk