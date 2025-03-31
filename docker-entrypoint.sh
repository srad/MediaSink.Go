#!/usr/bin/env bash

export $(egrep  -v '^#'  /run/secrets/secret| xargs)

./main