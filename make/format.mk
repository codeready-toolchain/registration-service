.PHONY: format
## format go code
format:
	gofmt -s -l -w $(shell find  . -name '*.go' | grep -vEf .gofmt_exclude)
