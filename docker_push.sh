#!/usr/bin/env bash

echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
docker push kleinhenz/registration-service:0.1
docker push kleinhenz/registration-service:latest
