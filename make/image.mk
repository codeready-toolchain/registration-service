IMAGE_NAME?=quay.io/${GO_PACKAGE_ORG_NAME}/${GO_PACKAGE_REPO_NAME}:latest

.PHONY: image
## Build the docker image locally that can be deployed (only contains bare registration-service)
image: build
	$(Q)docker build -f build/Dockerfile -t quay.io/${GO_PACKAGE_ORG_NAME}/${GO_PACKAGE_REPO_NAME}:${GIT_COMMIT_ID_SHORT} \
	 -t ${IMAGE_NAME} .
