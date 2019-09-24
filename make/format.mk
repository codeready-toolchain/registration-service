.PHONY: format
## format go code
format:
	gofmt -s -l -w $(shell find  . -name '*.go' | grep -vEf .gofmt_exclude)

check-go-format:
	@gofmt -s -l $(shell find  . -name '*.go' | grep -vEf .gofmt_exclude) 2>&1 \
		| tee /tmp/gofmt-errors \
		| read \
	&& echo "ERROR: These files differ from gofmt's style (run 'make format' to fix this):" \
	&& cat /tmp/gofmt-errors \
	&& exit 1 \
	|| true