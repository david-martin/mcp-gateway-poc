
##@ Gateway API resources

.PHONY: gateway-api-install
gateway-api-install: kustomize ## Install Gateway API CRDs
	$(KUSTOMIZE) build config/dependencies/gateway-api | kubectl apply -f -


.PHONY: gateway-install
gateway-install: kustomize ## Install Gateway API CRDs
	$(KUSTOMIZE) build config/dependencies/istio/gateway | kubectl apply -f -