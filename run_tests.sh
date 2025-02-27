#!/bin/bash

# Navigate to the project root directory
cd "$(dirname "$0")"

# Run tests for all packages
echo "Running all tests..."
go test ./... -v

# Check if tests passed
if [ $? -eq 0 ]; then
    echo "All tests passed!"
else
    echo "Some tests failed."
fi