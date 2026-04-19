#!/bin/bash

# Update script for runners

APP_NAME="runners"
INSTALL_PATH="/usr/local/bin/$APP_NAME"

echo "Updating $APP_NAME..."

# Step 1: Rebuild the binary
echo "Rebuilding $APP_NAME for Linux..."
GOOS=linux GOARCH=amd64 go build -o $APP_NAME main.go

if [ $? -ne 0 ]; then
    echo "Build failed! Update aborted."
    exit 1
fi

# Step 2: Replace the existing binary
echo "Installing new version to $INSTALL_PATH..."
sudo mv $APP_NAME $INSTALL_PATH
sudo chmod +x $INSTALL_PATH

if [ $? -eq 0 ]; then
    echo "Update successful! New version is now active."
    # Display help to verify installation
    $APP_NAME --help | head -n 1
else
    echo "Update failed. Please check your sudo permissions."
    exit 1
fi

echo "If you want to apply new resource management logic to existing runners, run: $APP_NAME update --all"
