#!/bin/bash

# use: segment.sh [input] [output] [t0 t1 t2 t3 ....]

fullpath="$1"
file=$(basename "$fullpath")
folder=$(dirname "$fullpath")
filename="${file%.*}"
concat="$folder/$filename.txt"
rm -f "$concat"
cnt=0
segment_count=$(ls -Uba1 | grep "$filename"_ | wc -l)
formatted=$(printf "%04d\n" "$segment_count")
output_file="$2"

cleanup() {
  rm -f "$concat"
  rm -f "$filename"_cut_*
}

# Always delete tmp files, also on crash
trap cleanup EXIT

cleanup

# iterate on two steps: "start end start end ..."
# start from third argument
for ((i=3; i<=$#; i+=2)); do
  start="${!i}"

  j=$((i+1))
  end="${!j}"

  output="$folder/$filename"_cut_"$cnt".mp4

  if ffmpeg -hide_banner -loglevel quiet -i "$1" -ss "$start" -to "$end" -codec copy "$output"; then
    echo "ok:$output"
  else
    echo "error:$output"
    cleanup
    exit 1
  fi
  echo "file '$output'" >> "$concat"

  cnt=$((cnt + 1))
done

if ffmpeg -hide_banner -loglevel quiet -f concat -safe 0 -i "$concat" -codec copy "$output_file"; then
  rm -f "$folder/$filename"_cut_*
  rm -f "$concat"
  echo "complete:$output_file"
else
  echo "error:$output_file"
  cleanup
  exit 1
fi

#ffmpeg 60 -i "$1" -to 5 -ss "$2" -codec copy "$i.seg.$cnt.mp4"
#$ ffmpeg -ss 120 -i input -t 5 -codec copy clip2.mkv
#$ echo "file 'clip1.mkv'" > concat.txt
#$ echo "file 'clip2.mkv'" >> concat.txt
#$ ffmpeg -f concat -i concat.txt -codec copy output.mkv
