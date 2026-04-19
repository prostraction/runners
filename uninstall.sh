#!/bin/bash

# Uninstall script for runners

APP_NAME="runners"
INSTALL_PATH="/usr/local/bin/$APP_NAME"
CONFIG_DIR="$HOME/.runners"

echo "Stopping and removing all running containers managed by $APP_NAME..."
# We try to use the app itself to clean up if it's still there
if command -v $APP_NAME &> /dev/null; then
    docker ps -a --filter "name=gh-runner-" -q | xargs -r docker stop
    docker ps -a --filter "name=gh-runner-" -q | xargs -r docker rm
fi

echo "Removing binary from $INSTALL_PATH..."
if [ -f "$INSTALL_PATH" ]; then
    sudo rm "$INSTALL_PATH"
    echo "Binary removed."
else
    echo "Binary not found in $INSTALL_PATH."
fi

read -p "Do you want to remove the configuration directory and all runner data at $CONFIG_DIR? (y/N): " confirm
if [[ $confirm == [yY] || $confirm == [yY][eE][sS] ]]; then
    echo "Removing configuration directory..."
    rm -rf "$CONFIG_DIR"
    echo "Configuration removed."
else
    echo "Configuration directory preserved."
fi

echo "Uninstallation complete."
