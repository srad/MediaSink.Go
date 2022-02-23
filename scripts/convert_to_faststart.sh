#!/bin/bash

#!/bin/bash

script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
echo "Current folder: '$(pwd)'"

dir_counter=1
folders=$(ls -d $1/*)
dir_total=$(ls -d $1/* | wc -l)

for d in $folders; do
  echo
  echo "($dir_counter/$dir_total) Converting in dir: '$d'"
  echo "==========================================================="
  cd "$d"

  file_counter=1
  files_total=$(ls *.mp4 | wc -l)

  # Generate previews for all MP4 files.
  for i in *.mp4; do
      echo "($file_counter/$files_total) $i -> ${i%.*}.fast.mp4 -> ${i%.*}.mp4"
      if ffmpeg -i "$i" -c copy -map 0 -movflags faststart -hide_banner -y -loglevel error -progress ${i%.*}.fast.mp4; then
        echo rm "${i%.*}.mp4"
        echo mv "${i%.*}.fast.mp4" "${i%.*}.mp4"
        echo
        echo "completed $i"
      else
        echo "error:$i"
      fi
    file_counter=$((file_counter+1))
  done
  cd ..

  dir_counter=$((dir_counter+1))
done