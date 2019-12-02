.PHONY: generate
## generates the asset bundle to be packaged with the binary
generate:
	go run -tags=dev pkg/static/assets_generate.go

.PHONY: copy-reg-service-deployment
copy-reg-service-deployment:
	cp ./deploy/deployment.yaml ../host-operator/deploy/registration-service/deployment.yaml