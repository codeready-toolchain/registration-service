.PHONY: copy-reg-service-template
copy-reg-service-template:
	$(Q)cp ./deploy/registration-service.yaml ../host-operator/deploy/registration-service/registration-service.yaml