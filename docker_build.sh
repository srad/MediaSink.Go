#!/bin/bash

#docker build -t streamsink-server . --no-cache
docker buildx create --name rambo
docker buildx use rambo
docker buildx build --push --platform linux/amd64,linux/arm64 -t registry.sedrad.com/streamsink-server:latest -t sedrad/streamsink-server:latest .