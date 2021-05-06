ETC_HOSTS=/etc/hosts

# to watch all namespaces, keep namespace empty
APP_NAMESPACE ?= $(LOCAL_TEST_NAMESPACE)

.PHONY: deploy-e2e
deploy-e2e: get-e2e-repo
## Deploy the e2e resources with the local 'registration-service' repository only
	$(MAKE) -C ${E2E_REPO_PATH} dev-deploy-e2e REG_REPO_PATH=${PWD}
