#!/bin/bash

set -e

echo "Starting slave simulator..."
go run cmd/slavesim/main.go --transport=tcp &
SLAVE_PID=$!

# Wait a moment for slave to start up
sleep 2

echo "Performing master write request..."
go run cmd/master/main.go --transport=tcp --fc=16 --address=9000 --float=123.456

sleep 1

echo "Performing master read request (FC4)..."
OUTPUT=$(go run cmd/master/main.go --transport=tcp --fc=4 --address=9000 --quantity=2)
echo "$OUTPUT"

# Extract float value from output
FLOAT_VALUE=$(echo "$OUTPUT" | grep "Float32 interpretation:" | awk '{printf "%.3f", $3}')

echo "Verifying read value..."
if [ -z "$FLOAT_VALUE" ]; then
    echo "ERROR: Could not extract float value from output"
    kill $SLAVE_PID 2>/dev/null || true
    exit 1
fi

# Compare with expected value (allow small floating point differences)
EXPECTED="123.456"
if [ "$FLOAT_VALUE" != "$EXPECTED" ]; then
    echo "ERROR: Expected $EXPECTED but got $FLOAT_VALUE"
    kill $SLAVE_PID 2>/dev/null || true
    exit 1
fi

echo "SUCCESS: Read value matches expected value ($FLOAT_VALUE)"

sleep 1

echo "Stopping slave simulator..."
sudo pkill -P $SLAVE_PID || true
kill $SLAVE_PID 2>/dev/null || true
wait $SLAVE_PID 2>/dev/null || true

echo "Integration test completed successfully!"
