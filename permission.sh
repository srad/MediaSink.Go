#!/bin/bash

export $(cat .env.local | xargs)
#groupadd -r appgroup && useradd --no-log-init -r -g appgroup appuser
chown -R appuser:appgroup "${DATA_PATH}"
chmod -R g+rws "${DATA_PATH}"