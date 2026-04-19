#!/bin/bash

# Build and Install script for runners-manager

APP_NAME="runners-manager"
INSTALL_PATH="/usr/local/bin/$APP_NAME"

echo "Building $APP_NAME for Linux..."
GOOS=linux GOARCH=amd64 go build -o $APP_NAME main.go

if [ $? -eq 0 ]; then
    echo "Build successful."
else
    echo "Build failed!"
    exit 1
fi

echo "Installing to $INSTALL_PATH..."
sudo mv $APP_NAME $INSTALL_PATH
sudo chmod +x $INSTALL_PATH

if [ $? -eq 0 ]; then
    echo "Installation successful! You can now use '$APP_NAME' command."
else
    echo "Installation failed. Make sure you have sudo privileges."
    exit 1
fi

# Create config directory for current user
mkdir -p ~/.runners-manager

echo "Setup complete."
