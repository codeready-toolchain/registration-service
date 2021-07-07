.PHONY: generate
## generates the asset bundle to be packaged with the binary
generate:
	go run -tags=dev pkg/static/assets_generate.go

.PHONY: copy-reg-service-template
copy-reg-service-template:
	cp ./deploy/registration-service.yaml ../host-operator/deploy/registration-service/registration-service.yaml