# By default the project should be build under GOPATH/src/github.com/<orgname>/<reponame>
GO_PACKAGE_ORG_NAME ?= codeready-toolchain
GO_PACKAGE_REPO_NAME ?= $(shell basename $$PWD)
GO_PACKAGE_PATH ?= github.com/${GO_PACKAGE_ORG_NAME}/${GO_PACKAGE_REPO_NAME}

export LDFLAGS=-X ${GO_PACKAGE_PATH}/pkg/configuration.Commit=${GIT_COMMIT_ID} -X ${GO_PACKAGE_PATH}/pkg/configuration.BuildTime=${BUILD_TIME}
goarch ?= $(shell go env GOARCH)

.PHONY: build build-prod build-dev

# builds the production binary
build: build-prod

# buils a development binary that has no bundled assets but reads them
# from the filesystem. Use only for development.
## builds development binary
build-dev:
	$(Q)CGO_ENABLED=0 GOARCH=${goarch} GOOS=linux \
		go build ${V_FLAG} -ldflags="${LDFLAGS}" \
		-tags dev \
		-o $(OUT_DIR)/bin/registration-service \
		cmd/main.go

# builds the production binary with bundled assets
## builds production binary
build-prod:
	$(Q)CGO_ENABLED=0 GOARCH=${goarch} GOOS=linux \
		go build ${V_FLAG} -ldflags="${LDFLAGS} -s -w" -trimpath \
		-o $(OUT_DIR)/bin/registration-service \
		cmd/main.go

.PHONY: vendor
vendor:
	$(Q)go mod vendor

.PHONY: verify-dependencies
## Runs commands to verify after the updated dependecies of toolchain-common/API(go mod replace), if the repo needs any changes to be made
verify-dependencies: tidy vet build test lint-go-code

.PHONY: tidy
tidy: 
	go mod tidy

.PHONY: vet
vet:
	go vet ./...
	
.PHONY: pre-verify
pre-verify:
	echo "No Pre-requisite needed"