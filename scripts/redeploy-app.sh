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
# Note: requires that the Container registry has a public route.
#       (see 'setup-registry.sh')
#------------------------------------------------------------------

# build the registration service
echo "📦 building the binary"
VERBOSE=0 make build
echo "✅ done"

echo "📦 building the image"
REGISTRY_ROUTE=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
TIMESTAMP=$(date +%s)
IMAGE_NAME=registration-service:dev-$TIMESTAMP
podman build -f build/Dockerfile -t $REGISTRY_ROUTE/$HOST_NS/$IMAGE_NAME .
echo "✅ done"

# copy/replace the binary into the pod's container
echo "🚚 pushing the image into the Container registry"
podman push --tls-verify=false $REGISTRY_ROUTE/$HOST_NS/$IMAGE_NAME
echo "✅ done"

# restart the process in the pod's container
INTERNAL_REGISTRY=image-registry.openshift-image-registry.svc:5000
echo "✏️ patching the deployment with image $INTERNAL_REGISTRY/$HOST_NS/$IMAGE_NAME"
oc patch deployment/registration-service --type='json' -p="[{\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/image\", \"value\":\"$INTERNAL_REGISTRY/$HOST_NS/$IMAGE_NAME\"}]"
# oc rollout restart deployment/registration-service

# Et voilà!
echo "👋 all done!"