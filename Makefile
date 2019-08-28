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

.PHONY: format
format:
	gofmt -s -l -w $(shell find  . -name '*.go' | grep -vEf .gofmt_exclude)

.PHONY: image
image:
	docker build -t kleinhenz/registration-service:0.1 .
	docker tag kleinhenz/registration-service:0.1 kleinhenz/registration-service:latest

generate:
	go run -tags=dev pkg/static/assets_generate.go

build: build-prod

build-dev:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go build -v ${LDFLAGS} -tags dev -o registration-service ${PACKAGE_NAME}/cmd

build-prod: generate
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go build -v ${LDFLAGS} -o registration-service ${PACKAGE_NAME}/cmd

clean:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		rm -f pkg/static/generated_assets.go && \
		rm -f registration-service

test: test-prod

test-dev:
	@echo TESTING with fs assets...
	@go test -count=1 -tags dev ./...

test-prod: generate
	@echo TESTING with bundled assets...
	go test -count=1 ./...
