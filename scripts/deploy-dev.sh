#!/usr/bin/env bash
set -e

#------------------------------------------------------------------
# Script to use when working on the registration service,
# for faster build/test cycles.
# 
# 1. Build the registration-service binary
# 2. Build the Docker image, tagged with a timestamp
# 3. Push to the Container Registry of the OpenShift cluster
# 4. Patch the deployment/registration-service to use the new image
#
# Notes: 
# 1. requires that the Container registry has a public route.
#       (see 'setup-registry.sh')
# 2. requires that the `HOST_NS` env var is set
#------------------------------------------------------------------

setup() {
    echo "🛠 exposing the registry using the default route"
    oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
    REGISTRY_ROUTE=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
    echo "registry route: https://$REGISTRY_ROUTE"
    podman login -u kubeadmin -p $(oc whoami -t) --tls-verify=false $REGISTRY_ROUTE

    # scale down the deployment to 1 replica (a single pod is enough when working on the UI)
    echo "⚙️ scaling down the registration-service"
    oc scale --replicas=1 deployment/registration-service -n $HOST_NS

    echo
    echo "ℹ️ you can now run the 'refresh' command to build and deploy the registration-service from your local repo"
}

refresh() {
    # Create a flag for when we need to build the "debug" images instead.
    if [[ "$1" == "debug" ]]
    then
      DEBUG=true
    else
      DEBUG=false
    fi

    # build the registration service
    echo "📦 building the binary"
    if [[ "${DEBUG}" = true ]]
    then
      VERBOSE=0 make build-dev
    else
      VERBOSE=0 make build
    fi
    echo "✅ done"

    echo "📦 building the image"
    REGISTRY_ROUTE=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
    TIMESTAMP=$(date +%s)
    IMAGE_NAME=registration-service:dev-$TIMESTAMP

    # Build the debug image with podman.
    if [[ "${DEBUG}" = true ]]
    then
      IMAGE="${REGISTRY_ROUTE}/${HOST_NS}/${IMAGE_NAME}" VERBOSE=0 make podman-image-debug
    else
      IMAGE="${REGISTRY_ROUTE}/${HOST_NS}/${IMAGE_NAME}" VERBOSE=0 make podman-image
    fi
    echo "✅ done"

    # copy/replace the binary into the pod's container
    echo "🚚 pushing the image into the Container registry"
    podman push --tls-verify=false $REGISTRY_ROUTE/$HOST_NS/$IMAGE_NAME
    echo "✅ done"

    # restart the process in the pod's container
    INTERNAL_REGISTRY=image-registry.openshift-image-registry.svc:5000
    echo "✏️ patching the deployment with the pushed image ${INTERNAL_REGISTRY}/${HOST_NS}/${IMAGE_NAME}"
    oc patch deployment/registration-service -n ${HOST_NS} --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"'"${INTERNAL_REGISTRY}/${HOST_NS}/${IMAGE_NAME}"'"}]'

    if [[ "${DEBUG}" = true ]]
    then
      echo "✏️ patching the deployment's command to execute the registration service with Delve instead"
      oc patch deployment/registration-service --namespace "${HOST_NS}" --type='json' --patch='[{"op": "replace", "path": "/spec/template/spec/containers/0/command", "value": ["dlv", "--listen=:50000", "--headless", "--continue", "--api-version=2", "--accept-multiclient", "exec", "/usr/local/bin/registration-service"]}'
    fi
    # oc rollout restart deployment/registration-service

    # Et voilà!
    echo "👋 all done!"
}

if [ -z "$HOST_NS" ]; then
    echo "HOST_NS is not set";
    exit 1;
fi

if declare -f "$1" > /dev/null
then
    # call arguments verbatim
    "$@"
else
    # Show a helpful error
    echo "'$1' is not a valid command" >&2
    echo "available commands:"
    echo "setup         Configure the deployment with a single pod"
    echo "refresh       Rebuild the registration-service and update the pod"
    echo "refresh debug Rebuild the registration service with Delve on it listening on port 50000"
    echo ""
    exit 1
fi