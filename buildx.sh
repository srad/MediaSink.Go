#!/bin/bash

docker buildx create --name rambo
docker buildx use rambo
docker buildx build --push --platform linux/amd64,linux/arm64 -t sedrad/mediasink-server .