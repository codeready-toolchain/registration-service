COV_DIR = coverage
GOPKGS = $(shell go list ./...)

.PHONY: test
## runs all tests with bundles assets
test: generate
	@-mkdir -p $(COV_DIR)
	@-rm -f $(COV_DIR)/coverage.txt
	go test -timeout 10m -coverprofile=profile.out -covermode=atomic -coverpkg=$$GOPKGS ./...
