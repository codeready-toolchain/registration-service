COV_DIR = coverage

.PHONY: test
## runs all tests with bundles assets
test: generate
	@-mkdir -p $(COV_DIR)
	@-rm -f $(COV_DIR)/coverage.txt
	go test -count=1 -coverprofile=$(COV_DIR)/profile.out -covermode=atomic ./...
ifeq (,$(wildcard $(COV_DIR)/profile.out))
	cat $(COV_DIR)/profile.out >> $(COV_DIR)/coverage.txt
	rm $(COV_DIR)/profile.out
endif 