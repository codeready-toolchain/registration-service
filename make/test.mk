.PHONY: test
## runs all tests with bundles assets
test: generate
	go test -count=1 ./...
