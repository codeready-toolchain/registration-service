#!/usr/bin/env bash

GIT_COMMIT_ID_SHORT=`git rev-parse --short HEAD`
if [[ -n `git status --porcelain --untracked-files=no` ]]; then
    # uncommitted changes - dirty
    GIT_COMMIT_ID_SHORT="$GIT_COMMIT_ID_SHORT-dirty"
fi

IMAGE_NAME="quay.io/codeready-toolchain/registration-service:$GIT_COMMIT_ID_SHORT"
LATEST="quay.io/codeready-toolchain/registration-service:latest"

echo "$DOCKER_PASSWORD" | docker login quay.io -u "$DOCKER_USERNAME" --password-stdin

docker push "$IMAGE_NAME"
docker push "$LATEST"
