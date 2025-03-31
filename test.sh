#!/bin/bash

# https://github.com/mattn/go-sqlite3/issues/803
export CGO_CFLAGS="-g -O2 -Wno-return-local-addr"
export DB_FILENAME="~/recordings/mediasink_test.sqlite3"
export DATA_DIR=".previews"
export DATA_DISK="/"
export NET_ADAPTER="eth2"
export REC_PATH="/recordings"

go test -v ./...