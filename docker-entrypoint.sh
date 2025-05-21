#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
# The environment variable your application will ultimately use for the secret.
# You can name this whatever your application expects, e.g., MY_APP_API_KEY, DB_PASSWORD.
# For this example, we'll call it APP_SECRET.
APP_SECRET_EXPORT_NAME="APP_SECRET"

# Option 1: Name of the environment variable the user can set.
USER_ENV_VAR_SECRET_NAME="SECRET"

# Option 2: Name of the Docker secret file the user can mount.
# The actual path inside the container will be /run/secrets/SECRET_FILE_NAME
DOCKER_SECRET_FILE_NAME="SECRET_FILE"
DOCKER_SECRET_FILE_PATH="/run/secrets/$DOCKER_SECRET_FILE_NAME"

# --- Logic to retrieve the secret ---
APP_SECRET_VALUE=""
SECRET_SOURCE_MESSAGE=""

# Check for the environment variable first (it takes precedence if both are set).
# The syntax ${!VAR_NAME} is indirect expansion, allowing us to use the value of USER_ENV_VAR_SECRET_NAME
# as the name of the variable to check.
USER_ENV_VAR_VALUE="${!USER_ENV_VAR_SECRET_NAME}"

if [ -n "$USER_ENV_VAR_VALUE" ]; then
  echo "Using secret from environment variable '$USER_ENV_VAR_SECRET_NAME'."
  APP_SECRET_VALUE="$USER_ENV_VAR_VALUE"
  SECRET_SOURCE_MESSAGE="environment variable '$USER_ENV_VAR_SECRET_NAME'"
# Else, if the environment variable was not set (or was empty), check for the Docker secret file.
elif [ -f "$DOCKER_SECRET_FILE_PATH" ]; then
  echo "Using secret from Docker secret file '$DOCKER_SECRET_FILE_PATH'."
  # Read the content of the secret file.
  # Using printf %s to avoid issues with trailing newlines from cat.
  APP_SECRET_VALUE=$(printf '%s' "$(cat "$DOCKER_SECRET_FILE_PATH")")
  SECRET_SOURCE_MESSAGE="Docker secret file '$DOCKER_SECRET_FILE_PATH' (mounted from secret '$DOCKER_SECRET_FILE_NAME')"
fi

# --- Validate and Export ---
# Check if a secret value was actually obtained.
if [ -z "$APP_SECRET_VALUE" ]; then
  echo "Error: Secret not provided or is empty." >&2
  echo "Please define the secret using EITHER:" >&2
  echo "  1. An environment variable named '$USER_ENV_VAR_SECRET_NAME' (e.g., docker run -e $USER_ENV_VAR_SECRET_NAME=\"your_secret_value\" ...)" >&2
  echo "  2. A Docker secret file named '$DOCKER_SECRET_FILE_NAME' (e.g., docker run --secret $DOCKER_SECRET_FILE_NAME ...)" >&2
  exit 1
fi

# Export the obtained secret value under the consistent name for your application.
export "$APP_SECRET_EXPORT_NAME"="$APP_SECRET_VALUE"
echo "Secret successfully sourced from $SECRET_SOURCE_MESSAGE and exported as '$APP_SECRET_EXPORT_NAME'."

# --- Execute the main application command ---
# This will run the CMD from your Dockerfile or arguments passed to `docker run`.
echo "Starting main application..."
exec "$@"