#!/bin/bash

file_counter=1
files_total=$(ls *.mp4 | wc -l)

# Generate previews for all MP4 files.
for i in *.mp4; do
  echo "($file_counter/$files_total) $i -> ${i%.*}.fast.mp4 -> ${i%.*}.mp4"
  if ffmpeg -i "$i" -c copy -map 0 -movflags faststart -y -hide_banner -loglevel error "${i%.*}.fast.mp4"; then
    rm "${i%.*}.mp4" -f
    mv "${i%.*}.fast.mp4" "${i%.*}.mp4"
    echo
    echo "completed $i"
  else
    echo "error:$i"
  fi
  file_counter=$((file_counter+1))
done
