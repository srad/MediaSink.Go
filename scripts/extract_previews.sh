#!/bin/bash

# ------------------------------------------------------------
# This script creates from an video previews
#
# Intermediate files will be written to /tmp and cleaned up
# after processing
# ------------------------------------------------------------

# ------------------------------------------------------------
# Arguments
# ------------------------------------------------------------

PROGNAME=$0

usage() {
  cat <<EOF >&2
Usage: $PROGNAME [-i <file>] [-c <file>] [-e <number>] [-h <number>] [-p <dir>] [-m <dir>] [-s <dir>]

-i <file>:   full path to file: -f /home/my/video.mp4
-c <file>:   CSV file to which the processed file information are written
-e <number>: extracted number of frames: -e 64
-h <number>: height of extracted frames
-p <dir>   : path to store the animated GIF
-m <dir>   : path to store the montage image
-s <dir>   : path to store the stripe image

EOF
  exit 1
}

# 2 * parameters
if [ ! "$#" = "14" ]; then
  echo "Wrong argument count"
  usage
fi

POSITIONAL=()
while [[ $# -gt 0 ]]; do
  key="$1"

  case $key in
  -i | --input)
    i="$2"
    shift # past argument
    shift # past value
    ;;
  -c | --csv)
    csv_file="$2"
    shift # past argument
    shift # past value
    ;;
  -e | --extract)
    extract_frame_count="$2"
    shift # past argument
    shift # past value
    ;;
  -h | --height)
    height="$2"
    shift # past argument
    shift # past value
    ;;
  -p | --preview)
    previews="$2"
    shift # past argument
    shift # past value
    ;;
  -m | --montage)
    montages="$2"
    shift # past argument
    shift # past value
    ;;
  -s | --stripe)
    stripes="$2"
    shift # past argument
    shift # past value
    ;;
  --default)
    DEFAULT=YES
    shift # past argument
    ;;
  *) # unknown option
    echo "Unknown paramter $1"
    usage
    POSITIONAL+=("$1") # save it in an array for later
    shift              # past argument
    ;;
  esac
done

f=$(basename "$i")
dir_name=$(dirname "$i")
#size=$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 -i "$i")
frame_rate=$(bc -l <<<$(ffprobe -i "$i" -v error -of csv=p=0 -select_streams v:0 -show_entries stream=r_frame_rate))
total_frames=$(ffprobe -threads 2 -v error -select_streams v:0 -count_packets -show_entries stream=nb_read_packets -of csv=p=0 "$i")
# select mod expects a non fractional input for calculation
frame_distance=$(bc -l <<<"scale=0; $total_frames/$extract_frame_count")

cleanup() {
  rm -f /tmp/"${f%.*}"_frames*
}

# Always delete tmp files, also on crash
trap cleanup EXIT

# Montage/Mosaic
cleanup # previous stuff
# https://ffmpeg.org/ffmpeg-filters.html

if ffmpeg -i "$f" -y -threads 8 -an -vf "select=not(mod(n\,$frame_distance)),scale=-1:$height,drawtext=fontfile=/usr/share/fonts/truetype/DMMono-Regular.ttf: text='%{pts\:gmtime\:0\:%H\\\\\:%M\\\\\:%S}': rate=$frame_rate: x=(w-tw)/2: y=h-(2*lh): fontsize=30: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1" -hide_banner -loglevel error -stats -vsync vfr /tmp/"${f%.*}"_frames%06d.png; then
  echo ok:ffmpeg
else
  echo error:ffmpeg
  exit 1
fi

# There might be one excess frame because of rounding/modulo.
frames=$(ls /tmp/"${f%.*}"_frames* | head -n$extract_frame_count | tr "\n" " ")

# ---------------------------------------------------------------------------------------
# Tile
# ---------------------------------------------------------------------------------------
if montage $frames -mode concatenate -quality 75 -tile 8x "$dir_name/$montages/${f%.*}".jpg; then
  echo ok:montage:"$dir_name/$montages/${f%.*}".jpg
else
  echo error:montage:"$dir_name/$montages/${f%.*}".jpg
  exit 1
fi

# ---------------------------------------------------------------------------------------
# Stripe
# ---------------------------------------------------------------------------------------
if montage $frames -mode concatenate -resize x$height -quality 75 -tile x1 "$dir_name/$stripes/${f%.*}".jpg; then
  echo ok:stripe:"$dir_name/$stripes/${f%.*}".jpg
else
  echo error:stripe:"$dir_name/$stripes/${f%.*}".jpg
  exit 1
fi

# ---------------------------------------------------------------------------------------
# GIF animation
# https://superuser.com/questions/556029/how-do-f-convert-a-video-to-gif-using-ffmpeg-with-reasonable-quality
# ---------------------------------------------------------------------------------------
#if convert -quiet -delay 75 -loop 0 +dither -remap $frames "$previews/${f%.*}".gif; then
#    echo ok:gif
#else
#    echo error:gif
#    exit 1
#fi

# ---------------------------------------------------------------------------------------
# Slideshow video preview
# https://superuser.com/questions/556029/how-do-f-convert-a-video-to-gif-using-ffmpeg-with-reasonable-quality
# ---------------------------------------------------------------------------------------
#if convert -delay 75 $frames "$previews/${f%.*}".mp4; then
#    echo ok:mp4
#else
#    echo error:mp4
#    exit 1
#fi

# ---------------------------------------------------------------------------------------
# Add information to csv_file
# ---------------------------------------------------------------------------------------
#if echo "$f,$frame_rate,$total_frames,$frame_distance,$extract_frame_count,$size,$height" >> "$csv_file"; then
#    echo ok:csv
#else
#    echo error:csv
#    exit 1
#fi

# ---------------------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------------------
if cleanup; then
  echo "ok:cleanup"
else
  echo "error:cleanup"
  exit 1
fi
