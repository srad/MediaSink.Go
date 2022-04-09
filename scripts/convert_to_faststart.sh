#!/bin/bash

#!/bin/bash

script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
echo "Current folder: '$(pwd)'"

dir_counter=1
folders=$(ls -d $1/*)
dir_total=$(ls -d $1/* | wc -l)

# skip first
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
      if /home/saman/src/StreamSink.Go/scripts/mp4_to_faststart.sh .; then
        echo "completed $i"
        cd .previews/videos
        if /home/saman/src/StreamSink.Go/scripts/mp4_to_faststart.sh .; then
          echo "ok previews $i"
        else
          echo "error preview $i"
        fi
        cd ..
        cd ..
        cd ..
      else
        echo "error:$i"
      fi
    file_counter=$((file_counter+1))
  done
  cd ..

  dir_counter=$((dir_counter+1))
done
