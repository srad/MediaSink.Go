#!/bin/bash

go install github.com/swaggo/swag/cmd/swag@latest
swag init

# https://github.com/mattn/go-sqlite3/issues/803
export CGO_CFLAGS="-g -O2 -Wno-return-local-addr"
VERSION=dev
COMMIT="$(git rev-parse --short HEAD)"
go mod vendor
go build -o ./main -ldflags="-X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT'" -mod=mod