COV_DIR = coverage
GOPKGS = $(shell go list ./... | grep -v /test)

.PHONY: test
## runs all tests with bundles assets
test: generate
	# test the test package seperately, no coverage report
	go test -count=1 ./test
	# test the rest of the code, with coverage report	
	@-mkdir -p $(COV_DIR)
	@-rm -f $(COV_DIR)/coverage.txt
	@-for d in $(GOPKGS) ; do \
	  go test -coverprofile=profile.out -covermode=atomic -coverpkg=$$GOPKGS $$d ; \
    if [ -f profile.out ]; then \
      cat profile.out >> $(COV_DIR)/coverage.txt ; \
      rm profile.out ; \
    fi ; \
	done

	@#go test -count=1 -coverprofile=$(COV_DIR)/coverage.txt -covermode=atomic $(shell go list ./... | grep -v /test)
