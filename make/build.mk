# By default the project should be build under GOPATH/src/github.com/<orgname>/<reponame>
GO_PACKAGE_ORG_NAME ?= $(shell basename $$(dirname $$PWD))
GO_PACKAGE_REPO_NAME ?= $(shell basename $$PWD)
GO_PACKAGE_PATH ?= github.com/${GO_PACKAGE_ORG_NAME}/${GO_PACKAGE_REPO_NAME}

COMMIT=$(shell git rev-parse HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	COMMIT := $(COMMIT)-dirty
endif
BUILD_TIME=`date -u '+%Y-%m-%dT%H:%M:%SZ'`
export LDFLAGS=-ldflags "-X ${GO_PACKAGE_PATH}/pkg/configuration.Commit=${COMMIT} -X ${GO_PACKAGE_PATH}/cmd/configuration.BuildTime=${BUILD_TIME}"

.PHONY: build build-prod build-dev

# builds the production binary
build: build-prod

# buils a development binary that has no bundled assets but reads them
# from the filesystem. Use only for development.
## builds development binary
build-dev:
	$(Q)CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
		go build ${V_FLAG} ${LDFLAGS} \
		-tags dev \
		-o $(OUT_DIR)/bin/registration-service \
		cmd/main.go

# builds the production binary with bundled assets
## builds production binary
build-prod: generate
	$(Q)CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
		go build ${V_FLAG} ${LDFLAGS} \
		-o $(OUT_DIR)/bin/registration-service \
		cmd/main.go

.PHONY: vendor
vendor:
	$(Q)go mod vendor