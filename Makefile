export GO111MODULE=on

# It's necessary to set this because some environments don't link sh -> bash.
SHELL := /bin/bash

include ./make/*.mk
.DEFAULT_GOAL := help


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
#GO_FILES=$(shell find . -type f -name '*.go')





#.phony: all depend test build clean

#all: test build
#	@echo DONE!










