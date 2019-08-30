.PHONY: image
## build docker image
image:
	docker build -t kleinhenz/registration-service:0.1 .
	docker tag kleinhenz/registration-service:0.1 kleinhenz/registration-service:latest
