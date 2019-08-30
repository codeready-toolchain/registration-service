#! /usr/bin/make
#
# Makefile for registration-service
#
# Targets:
# - "depend" retrieves the Go packages needed to run the tests
# - "build" compiles the microservice
# - "clean" deletes the generated files
# - "test" runs the tests
#
# Meta targets:
# - "all" is the default target, it runs all the targets in the order above.
#
GO_FILES=$(shell find . -type f -name '*.go')

export GO111MODULE=on

COMMIT=$(shell git rev-parse HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	COMMIT := $(COMMIT)-dirty
endif
BUILD_TIME=`date -u '+%Y-%m-%dT%H:%M:%SZ'`
PACKAGE_NAME := github.com/codeready-toolchain/registration-service
export LDFLAGS=-ldflags "-X ${PACKAGE_NAME}/pkg/configuration.Commit=${COMMIT} -X ${PACKAGE_NAME}/pkg/configuration.BuildTime=${BUILD_TIME}"

# Only list test and build dependencies
# Standard dependencies are installed via go get
DEPEND=\
	github.com/gorilla/mux \
	github.com/shurcooL/vfsgen

.phony: all depend test build clean

all: test build
	@echo DONE!

depend:
	@echo INSTALLING DEPENDENCIES...
	@env GO111MODULE=off go get -v $(DEPEND)

# formats go code
.PHONY: format
format:
	gofmt -s -l -w $(shell find  . -name '*.go' | grep -vEf .gofmt_exclude)

# builds docker image
.PHONY: image
image:
	docker build -t kleinhenz/registration-service:0.1 .
	docker tag kleinhenz/registration-service:0.1 kleinhenz/registration-service:latest

# generates the asset bundle to be packaged with the binary
generate:
	go run -tags=dev pkg/static/assets_generate.go

# builds the production binary
build: build-prod

# buils a development binary that has no bundled assets but reads them
# from the filesystem. Use only for development.
build-dev:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go build -v ${LDFLAGS} -tags dev -o registration-service ${PACKAGE_NAME}/cmd

# builds the production binary with bundled assets
build-prod: generate
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go build -v ${LDFLAGS} -o registration-service ${PACKAGE_NAME}/cmd

# cleans up, removes generated asset bundle
clean:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		rm -f pkg/static/generated_assets.go && \
		rm -f registration-service

# runs all tests with bundled assets
test: generate
	go test -count=1 ./...
