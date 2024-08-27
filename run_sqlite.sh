#!/bin/bash

#rm streamsink.sqlite3
./build.sh
REC_PATH= DATA_DIR=".previews" DB_FILENAME= ./streamsink
