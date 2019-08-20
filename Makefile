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

build: build-prod

build-dev:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go build -tags dev

build-prod:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		go generate && \
		go build

clean:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		rm -f static/generated_assets.go && \
		rm -f registration-service

test: test-prod

test-dev:
	@echo TESTING with fs assets...
	@go test -tags dev ./...

test-prod:
	@echo TESTING with bundled assets...
	@go generate && \
	go test ./...
