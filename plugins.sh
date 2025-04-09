#!/bin/bash

# --- Configuration ---
# Source directory containing subdirectories for each plugin
SOURCE_DIR="pluginsrc"
# Destination directory for compiled .so files
DEST_DIR="plugins"

# Exit script immediately if any command fails
set -e

# --- Validation ---
if [ ! -d "$SOURCE_DIR" ]; then
  echo "Error: Source directory '$SOURCE_DIR' not found."
  echo "Please ensure it exists and contains subdirectories for each plugin."
  exit 1
fi

# --- Preparation ---
# Create the destination directory if it doesn't exist
mkdir -p "$DEST_DIR"
# Get the absolute path to the destination directory for reliable output paths
ABS_DEST_DIR=$(cd "$DEST_DIR"; pwd)

echo "Starting plugin compilation..."
echo "Source: $(pwd)/$SOURCE_DIR"
echo "Output: $ABS_DEST_DIR"
echo "----------------------------------------"

# Ensure loop doesn't run if no directories match
shopt -s nullglob

found_plugins=0
compile_errors=0

# --- Compilation Loop ---
# Loop through each subdirectory in the SOURCE_DIR
# The trailing slash ensures we only match directories
for dir in "$SOURCE_DIR"/*/; do
  # Check if it is indeed a directory (good practice though the glob does this)
  if [ -d "$dir" ]; then
    plugin_name=$(basename "$dir") # Extract directory name (e.g., "sendemail")
    output_file="$ABS_DEST_DIR/${plugin_name}.so" # Construct the output .so file path

    echo "--> Compiling plugin: '$plugin_name'"
    echo "    Source: $dir"

    found_plugins=$((found_plugins + 1))

    # Compile the plugin. Run the build command inside the plugin's directory.
    # Using a subshell (parentheses) keeps 'cd' local to this iteration.
    if (cd "$dir" && go build -v -buildmode=plugin -o "$output_file" .); then
      # Check if the output file was actually created (basic sanity check)
      if [ -f "$output_file" ]; then
        echo "    Success: Output -> $output_file"
      else
         echo "    ERROR: Build reported success, but output file '$output_file' not found."
         compile_errors=$((compile_errors + 1))
      fi
    else
      # go build command failed
      echo "    ERROR: Failed to compile plugin '$plugin_name'."
      compile_errors=$((compile_errors + 1))
      # set -e will cause script exit here if uncommented earlier,
      # otherwise execution continues to the next plugin
    fi
     echo "----------------------------------------"
  fi
done

# Turn off nullglob behavior
shopt -u nullglob


# --- Summary ---
echo "Plugin compilation finished."
if [ $found_plugins -eq 0 ]; then
  echo "Warning: No plugin source directories found in '$SOURCE_DIR/*/'."
else
  echo "Processed $found_plugins potential plugin director(y/ies)."
  if [ $compile_errors -eq 0 ]; then
    echo "All plugins compiled successfully!"
  else
    echo "$compile_errors plugin(s) failed to compile."
    exit 1 # Exit with error status if any plugin failed
  fi
fi

exit 0