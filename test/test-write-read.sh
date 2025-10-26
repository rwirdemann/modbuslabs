#!/bin/bash

SCRIPTDIR="$(cd -- "$(dirname -- "$0")" >/dev/null 2>&1 && pwd)"

set -e

cd "${SCRIPTDIR}/.." || exit 1

# Parse transport parameter (default: tcp)
TRANSPORT="tcp"
while [ $# -gt 0 ]; do
  case "$1" in
    --transport=*)
      TRANSPORT="${1#*=}"
      shift
      ;;
    --transport)
      TRANSPORT="$2"
      shift 2
      ;;
    *)
      echo "Unknown parameter: $1"
      exit 1
      ;;
  esac
done

echo "Using transport: ${TRANSPORT}"

go run cmd/slavesim/main.go --transport="${TRANSPORT}" > /dev/null 2>&1 &
SLAVE_PID=$!

go run cmd/master/main.go --transport="${TRANSPORT}" --fc=5 --address=0x7E22 --value=true
go run cmd/master/main.go --transport="${TRANSPORT}" --fc=16 --address=9000 --value=123.456
go run cmd/master/main.go --transport="${TRANSPORT}" --fc=6 --address=8000 --value=42

OUTPUT=$(go run cmd/master/main.go --transport="${TRANSPORT}" --fc=4 --address=9000 --quantity=2) > /dev/null 2>&1
echo "$OUTPUT"

BOOL_OUTPUT=$(go run cmd/master/main.go --transport="${TRANSPORT}" --fc=2 --address=0x7E22 2>/dev/null)

UINT16_OUTPUT=$(go run cmd/master/main.go --transport="${TRANSPORT}" --fc=4 --address=8000 --quantity=1)

FLOAT_VALUE=$(echo "$OUTPUT" | grep "Float32 interpretation:" | awk '{printf "%.3f", $3}')

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

# Check boolean value
BOOL_VALUE=$(echo "$BOOL_OUTPUT" | grep -o "true\|false")
if [ "$BOOL_VALUE" != "true" ]; then
    echo "ERROR: Expected boolean true but got $BOOL_VALUE"
    kill $SLAVE_PID 2>/dev/null || true
    exit 1
fi

# Check uint16 value
UINT16_VALUE=$(echo "$UINT16_OUTPUT")
EXPECTED_UINT16="42"
if [ "$UINT16_VALUE" != "$EXPECTED_UINT16" ]; then
    echo "ERROR: Expected uint16 $EXPECTED_UINT16 but got $UINT16_VALUE"
    kill $SLAVE_PID 2>/dev/null || true
    exit 1
fi

sleep 1

sudo pkill -P $SLAVE_PID || true
kill $SLAVE_PID 2>/dev/null || true
wait $SLAVE_PID 2>/dev/null || true

echo "test completed successfully!"
