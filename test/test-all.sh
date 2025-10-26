#!/bin/bash

# Cleanup function to stop virtual ports
cleanup() {
    if [ -n "$SETUP_PID" ]; then
        echo ""
        echo "Stopping virtual ports (PID: $SETUP_PID)..."
        kill $SETUP_PID 2>/dev/null
        wait $SETUP_PID 2>/dev/null
    fi
}

# Set trap to ensure cleanup runs on exit
trap cleanup EXIT INT TERM

# Start virtual ports setup in background
echo "Starting virtual ports setup..."
./setup-virtual-ports.sh > /dev/null 2>&1 &
SETUP_PID=$!

# Wait a moment for ports to be created
sleep 2
echo "Virtual ports ready (PID: $SETUP_PID)"
echo ""

# Transport modes to test
transports=("tcp" "rtu")

# Find and execute all test-*.sh scripts in the current directory
for test_script in test-*.sh; do
    # Skip if no matching files found
    if [ "$test_script" = "test-*.sh" ]; then
        echo "No test scripts found"
        exit 0
    fi

    # Skip test-all.sh itself
    if [ "$test_script" = "test-all.sh" ]; then
        continue
    fi

    # Run each test with both transport modes
    for transport in "${transports[@]}"; do
        echo "Running $test_script --transport=$transport..."
        if bash "$test_script" --transport="$transport"; then
            echo "✓ $test_script --transport=$transport passed"
        else
            echo "✗ $test_script --transport=$transport failed"
            exit 1
        fi
        echo ""
    done
done

echo "All tests completed successfully"
