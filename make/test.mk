COV_DIR = coverage

.PHONY: test
## runs all tests with bundles assets
test: generate
	@-mkdir -p $(COV_DIR)
	@-rm -f $(COV_DIR)/coverage.txt
	# test the test package seperately, no coverage report
	go test -count=1 ./test
	# test the rest of the code, with coverage report
	go test -count=1 -coverprofile=$(COV_DIR)/coverage.txt -covermode=atomic $(shell go list ./... | grep -v /test)
