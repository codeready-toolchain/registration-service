#!/usr/bin/env bash
set -e

#------------------------------------------------------------------
# Configure a public route to access the Container Registry 
# of the OpenShift cluster.
# (required before build/pushing new images with `redeploy-app.sh`)
#------------------------------------------------------------------
echo "🛠 exposing the registry using the default route"
oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
REGISTRY_ROUTE=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
echo "registry route: https://$REGISTRY_ROUTE"
podman login -u kubeadmin -p $(oc whoami -t) --tls-verify=false $REGISTRY_ROUTE

# scale down the deployment to 1 replica (a single pod is enough when working on the UI)
echo "⚙️ scaling down the registration-service"
oc scale --replicas=1 deployment/registration-service