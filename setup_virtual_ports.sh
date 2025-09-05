#!/bin/bash

echo "Virtual Serial Port Setup for macOS Testing"
echo "==========================================="
echo ""

if ! command -v socat &> /dev/null; then
    echo "socat is not installed. Installing via Homebrew..."
    if ! command -v brew &> /dev/null; then
        echo "Homebrew is not installed. Please install Homebrew first:"
        echo "/bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
        exit 1
    fi
    brew install socat
fi

echo "Creating virtual serial port pair..."
echo "Port 1: /tmp/virtualcom0 (for Modbus slave)"
echo "Port 2: /tmp/virtualcom1 (for test client)"
echo ""
echo "Starting socat to create virtual serial port pair..."
echo "Press Ctrl+C to stop"
echo ""

socat -d -d pty,raw,echo=0,link=/tmp/virtualcom0 pty,raw,echo=0,link=/tmp/virtualcom1