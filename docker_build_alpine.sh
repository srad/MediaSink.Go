#!/bin/bash

docker build -f Dockerfile-alpine -t streamsink-server:alpine . --no-cache
