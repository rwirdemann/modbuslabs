#!/bin/bash

# Integration test script for modbuslabs
# Starts slave simulator, performs master write request, then stops simulator

set -e

echo "Starting slave simulator..."
go run cmd/slavesim/main.go --transport=tcp &
SLAVE_PID=$!

# Wait a moment for slave to start up
sleep 2

echo "Performing master write request..."
go run cmd/master/main.go --transport=tcp --fc=6 --address=3 --value=123

echo "Stopping slave simulator..."
pkill -P $SLAVE_PID || true
kill $SLAVE_PID 2>/dev/null || true
wait $SLAVE_PID 2>/dev/null || true

echo "Integration test completed successfully!"
