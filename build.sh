#!/bin/bash

go install github.com/swaggo/swag/cmd/swag
swag init
# https://github.com/mattn/go-sqlite3/issues/803
export CGO_CFLAGS="-g -O2 -Wno-return-local-addr"
go build -o streamsink
