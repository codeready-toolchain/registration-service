#!/usr/bin/env bash
set -e

#-----------------------------------------------------------------------
# Deploy the Registration Service in "dev mode":
#
# 1. Process the template with the following changes:
#   a. set `IMAGE` to `quay.io/app-sre/ubi8-ubi:latest`
#   b. set `REPLICAS` to 1
# 2. Apply the template with Kustomize to:
#   a. remove the liveness and healthcheck probes from the container spec
#   b. set the container command to `sleep 36000` (10hrs)
#-----------------------------------------------------------------------

DIRNAME=$(dirname $0) # path to this script

setup() {
    echo "♻️ processing the registration-service.yaml template"
    oc process \
        -f $DIRNAME/../deploy/registration-service.yaml \
        --local \
        -o yaml > $DIRNAME/registration-service.yaml
    echo "♻️ kustomizing and applying the registration-service deployment"
    oc apply -k $DIRNAME 
    echo "✅ done"
}

refresh() {
    # build the registration service
    echo "📦 building the binary"
    VERBOSE=0 make build
    echo "✅ done"

    PODNAME=`oc get pods -l name=registration-service -o json | jq -r '.items[0].metadata.name'`
    echo "🚚 copying the binary into the $PODNAME pod"
    oc cp ./build/_output/bin/registration-service $PODNAME:/tmp
    echo "☠️  stopping the current registration-service process (if applicable)"
    oc exec $PODNAME -- killall registration-service || true
    echo "🏃 running the new registration-service"
    oc exec $PODNAME -- /tmp/registration-service &
    echo "✅ done"
}

if declare -f "$1" > /dev/null
then
    # call arguments verbatim
    "$@"
else
    # Show a helpful error
    echo "'$1' is not a valid command" >&2
    echo "available commands:"
    echo "setup     Configure the deployment with a single pod"
    echo "refresh   Rebuild the registration-service and update the pod"
    echo ""
    exit 1
fi


# Et voilà!
echo "👋 all done!"