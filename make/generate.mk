.PHONY: generate
## generates the asset bundle to be packaged with the binary
generate: frontend
	go run -tags=dev pkg/static/assets_generate.go
