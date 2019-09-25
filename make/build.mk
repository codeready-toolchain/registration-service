COMMIT=$(shell git rev-parse HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	COMMIT := $(COMMIT)-dirty
endif
BUILD_TIME=`date -u '+%Y-%m-%dT%H:%M:%SZ'`
PACKAGE_NAME := github.com/codeready-toolchain/registration-service
export LDFLAGS=-ldflags "-X ${PACKAGE_NAME}/pkg/configuration.Commit=${COMMIT} -X ${PACKAGE_NAME}/pkg/configuration.BuildTime=${BUILD_TIME}"

.PHONY: build build-prod build-dev

# builds the production binary
build: build-prod

# buils a development binary that has no bundled assets but reads them
# from the filesystem. Use only for development.
## builds development binary
build-dev:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go build -v ${LDFLAGS} -tags dev -o registration-service ${PACKAGE_NAME}/cmd

# builds the production binary with bundled assets
## builds production binary
build-prod: generate
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go build -v ${LDFLAGS} -o registration-service ${PACKAGE_NAME}/cmd

.PHONY: vendor
vendor:
	$(Q)go mod vendor