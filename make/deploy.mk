
.PHONY: deploy-registration-service
## Deploy Registration service on dev cluster
deploy-registration-service: create-namespace deploy-rbac
	oc apply -f deploy/deployment_dev_cluster.yaml
