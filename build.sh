#!/bin/bash

set -e

echo "LoadTestForge Quick Start Script"
echo "================================="
echo ""

if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

echo "Step 1: Installing dependencies..."
go mod download
go mod tidy

echo "Step 2: Running tests..."
go test -v ./...

echo "Step 3: Building binary..."
go build -o loadtest ./cmd/loadtest

echo ""
echo "Build complete!"
echo ""
echo "Run a test:"
echo "  ./loadtest --target http://httpbin.org/get --sessions 100 --rate 10 --duration 30s"
echo ""
echo "For more options:"
echo "  ./loadtest --help"
echo ""
