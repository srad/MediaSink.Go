#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$1"
echo "folder:'$(pwd)'"

height=200

# 1 minute
extracted_frames=64

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
touch "$csv"

counter=1
total=$(ls *.mp4 | wc -l)

# Generate previews for all MP4 files.
for i in *.mp4; do
  cnt=$(< "$csv" grep -c "$i")

  if [ "$cnt" -gt 0 ]; then
    echo "message:Preview already generated,$i"
  else
    echo "start:$i,$counter,$total"
    if "$SCRIPT_DIR"/extract_previews.sh -i "$1$i" -c "$csv" -e "$extracted_frames" -h "$height" -p "$previews" -m "$montages" -s "$stripes"; then
      echo "message:completed,$i"
    else
      echo "error:$i"
    fi
  fi
  counter=$((counter+1))
done
