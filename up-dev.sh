#!/bin/bash

UID=$(id -u appuser)
GID=$(getent group appgroup | cut -d: -f3)
export UID
export GID
echo "Starting with UID=$UID and GID=$GID"
export $(cat .env.local | xargs)
docker compose down
docker compose up --build