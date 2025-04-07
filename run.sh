#!/bin/bash

#rm mediasink.sqlite3
./build.sh

export DB_FILENAME="mediasink.sqlite3"
export DATA_DIR=".previews"
export DATA_DISK="/"
export NET_ADAPTER="eth0"
export REC_PATH="/home/saman/recordings"

./main
