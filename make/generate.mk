.PHONY: generate
## generates the asset bundle to be packaged with the binary
generate:
	go run -tags=dev pkg/static/assets_generate.go
