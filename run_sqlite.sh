#!/bin/bash

#rm streamsink.sqlite3
./build.sh
REC_PATH=/home/saman/recordings DATA_DIR=".previews" DB_FILENAME=/home/saman/recordings/streamsink.sqlite3 ./streamsink
