#!/bin/bash

echo "🚀 Building Siam-Synapse Worker for Android/Termux (ARM64)..."

# Termux runs a Linux environment on Android.
# GOOS=linux GOARCH=arm64 is the standard target for Android Termux.
# Sometimes CGO is an issue on Android, so we disable it just to be safe for a portable binary.
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o siam-worker-termux main.go

if [ $? -eq 0 ]; then
    echo "✅ Build complete: siam-worker-termux"
    echo "📦 You can now copy this file to your phone and run it in Termux!"
    echo "Example Usage in Termux:"
    echo "  export MASTER_GRPC_URL=100.98.34.45:50051"
    echo "  export NODE_ID=termux-phone-01"
    echo "  ./siam-worker-termux"
else
    echo "❌ Build failed."
fi
