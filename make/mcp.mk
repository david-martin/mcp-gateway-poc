
.PHONY: mcp-server-install
mcp-server-install: kustomize ## Install Gateway API CRDs
	$(KUSTOMIZE) build config/dependencies/mock-mcp | kubectl apply -f -