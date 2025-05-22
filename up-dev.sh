#!/bin/bash

export $(cat .env.local | xargs)
echo "Creating directory: ${DATA_PATH} ..."
mkdir -p "${DATA_PATH}"
docker compose down
docker compose --env-file=.env.local up --build