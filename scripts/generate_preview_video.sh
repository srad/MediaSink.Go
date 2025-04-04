#!/bin/bash

# ----------------------------------------------------------------------------
# generate_preview_video.sh shell script:
#
# This script generates a short preview video from an input video. It creates
# a preview by selecting NUM_CLIPS segments of length SEGMENT_DURATION from the
# video and speeding them up. The segments are chosen evenly throughout the
# entire video.
#
# Parameters:
#   $1 - input video file (e.g., input.mp4)
#   $2 - output video file (e.g., output.mp4)
#   $3 - Height of the video, the width will be scaled proportionally
#
# Example:
#   ./generate_preview_video.sh input.mp4 output.mp4 480
#
# The script performs the following tasks:
# 1. It calculates the duration of the input video.
# 2. It checks if the input video is shorter than the preview duration. If so,
#    the script exits without creating a preview.
# 3. It generates a selection of segments, applies a speed-up effect, and scales
#    them to a predefined height (240px).
# 4. It outputs the generated preview to the specified output file.
# ----------------------------------------------------------------------------

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 input.mp4 output.mp4"
    exit 1
fi

# Calculate half of the available cores
HALF_CORES=$(($(nproc)/2))

INPUT_FILE="$1"
OUTPUT_FILE="$2"
HEIGHT="$3"
NUM_CLIPS=25
SEGMENT_DURATION=2

OUTPUT_DURATION=$((NUM_CLIPS * SEGMENT_DURATION))
OUTPUT_DURATION=${OUTPUT_DURATION%.*} # floor

DURATION=$(ffprobe -v error -show_entries format=duration -of csv="p=0" "$INPUT_FILE")
DURATION=${DURATION%.*} # floor

cleanup() {
  rm -f "${OUTPUT_FILE}"
}

# Crash cleanup
trap cleanup EXIT

# Has the video the preview length? Then just emit
if [ "${DURATION}" -le "${OUTPUT_DURATION}" ]; then
  ffmpeg -hide_banner -loglevel error -stats \
         -i "${INPUT_FILE}" -y -threads "${HALF_CORES}" \
         -progress - -nostats \
         -vf "setpts=N/FRAME_RATE/TB*0.65,scale=-2:${HEIGHT}" \
         -an "${OUTPUT_FILE}"
  exit 0
fi

INTERVAL=$((DURATION / NUM_CLIPS))

# Create select expressions
SELECT_EXPR=""
for ((i=0; i<NUM_CLIPS; i++)); do
    START=$(( i * INTERVAL ))
    END=$(( START + SEGMENT_DURATION ))
    SELECT_EXPR+="between(t\,${START}\,${END})+"
done

# Remove trailing '+'
SELECT_EXPR=${SELECT_EXPR%+}

ffmpeg -hide_banner -loglevel error -stats \
       -i "${INPUT_FILE}" -y -threads "${HALF_CORES}" \
       -progress - -nostats \
       -vf "select='${SELECT_EXPR}',setpts=N/FRAME_RATE/TB*0.65,scale=-2:${HEIGHT}" \
       -an "${OUTPUT_FILE}"