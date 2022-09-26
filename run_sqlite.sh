#!/bin/bash

rm streamsink.sqlite3
go build -o streamsink
./streamsink
