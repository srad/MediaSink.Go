#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd $(dirname $1)

height=150

# 1 minute
extracted_frames=100

previews=.previews/previews
montages=.previews/montages
stripes=.previews/stripes
csv=.previews/info.csv

# Recreate all files
if [ $# -eq 2 ] && [ "$2" = "-f" ]; then
  echo "Force recreate"
  rm -fr .previews/
  rm -fr .previews
  rm -fr .montages
  rm -fr .stripes
  rm -f .previews
fi

# Setup
#mkdir -p "$previews" # GIF disabled for now
mkdir -p "$montages"
mkdir -p "$stripes"
#touch "$csv"

echo "start:$1"

if "$SCRIPT_DIR"/extract_previews.sh -i "$1" -c "$csv" -e "$extracted_frames" -h "$height" -p "$previews" -m "$montages" -s "$stripes"; then
  echo "complete:$1"
else
  echo "error:$1"
fi
