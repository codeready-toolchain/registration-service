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

    # Get the CSV for the host operator, in order to be able to patch it.
    HOST_CSV_NAME=$(oc get --namespace="${HOST_NS}" --output name ClusterServiceVersion)
    oc patch --namespace="${HOST_NS}" "${HOST_CSV_NAME}" --type='json' --patch='[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/1/env/0", "value": {"name": "REGISTRATION_SERVICE_IMAGE", "value": "'"${INTERNAL_REGISTRY}/${HOST_NS}/${IMAGE_NAME}"'"}}]'

    # Wait for the registration service's image to be replaced.
    oc wait --namespace="${HOST_NS}" --timeout=3m --for=jsonpath='{.spec.template.spec.containers[0].image}'="${INTERNAL_REGISTRY}/${HOST_NS}/${IMAGE_NAME}" "deployment/registration-service"
    oc rollout status --namespace="${HOST_NS}" --timeout=3m deployment/registration-service

    if [[ "${DEBUG}" = true ]]
    then
      echo "✏️ patching the deployment's command to execute the registration service with Delve instead"
      	if ! oc get --namespace="${HOST_NS}" "${HOST_CSV_NAME}" --output jsonpath="{.spec.install.spec.deployments[0].spec.template.spec.containers[1].env}" | grep -q "REGISTRATION_SERVICE_COMMAND"; then \
      		oc patch --namespace="${HOST_NS}" "${HOST_CSV_NAME}" --type='json' --patch='[{"op": "add", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/1/env/-", "value": {"name": "REGISTRATION_SERVICE_COMMAND", "value": "[\"dlv\", \"--listen=:50000\", \"--headless\", \"--continue\", \"--api-version=2\", \"--accept-multiclient\", \"exec\", \"/usr/local/bin/registration-service\"]"}}]'

      		# Wait for the registration service's command to have the "dlv" bit, and the rollout for its deployment to be
      		# complete.
      		echo "Waiting for the registration service's deployment to get updated..."
      		oc wait --namespace="${HOST_NS}" --timeout=3m --for=jsonpath='{.spec.template.spec.containers[0].command[0]}'="dlv" "deployment/registration-service"
      		oc rollout status --namespace="${HOST_NS}" --timeout=3m deployment/registration-service
      	fi
      oc patch deployment/registration-service --namespace "${HOST_NS}" --type='json' --patch='[{"op": "replace", "path": "/spec/template/spec/containers/0/command", "value": ["dlv", "--listen=:50000", "--headless", "--continue", "--api-version=2", "--accept-multiclient", "exec", "/usr/local/bin/registration-service"]}]'
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
