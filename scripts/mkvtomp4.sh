#!/bin/bash
cd "${1%/*}"

for d in */ ; do
  counter=1
  cd "$d"
  total=$(ls *.mkv | wc -l)
  for i in *.mkv; do
      echo "($counter/$total) Processing '$i'"
      ffmpeg -hide_banner -loglevel error -threads 0 -i "$i" -codec copy "${i%.*}.mp4"
      rm "$i"
      counter=$((counter+1))
  done
  cd ..
done
