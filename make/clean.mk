.PHONY: clean
## cleans up, removes generated asset bundle
clean:
	@cd "$(GOPATH)/src/github.com/codeready-toolchain/registration-service" && \
		rm -f pkg/static/generated_assets.go && \
		rm -rf $(COV_DIR) && \
		rm -f registration-service
