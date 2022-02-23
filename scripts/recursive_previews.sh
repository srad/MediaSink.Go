#!/bin/bash

script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
echo "Current folder: '$(pwd)'"

counter=1
folders=$(ls -d $1/*)
total=$(ls -d $1/* | wc -l)

for d in $folders; do
  echo "($counter/$total) Generating preview for '$d'"
  "$script_dir/generate_previews.sh $d"
  counter=$((counter+1))
done
