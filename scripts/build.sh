#!/bin/bash

# Timeclip Build Script
set -e

echo "🔨 Building Timeclip..."

# Get the current directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Build the application
echo "📦 Compiling Go application..."
go build -o timeclip cmd/timeclip/main.go

# Make executable
chmod +x timeclip

echo "✅ Build completed successfully!"
echo "📍 Binary location: $PROJECT_DIR/timeclip"

# Show file info
ls -la timeclip

echo ""
echo "🚀 To run: ./timeclip"
echo "⚙️  Config location: ~/.timeclip/config.toml"
echo "💾 Database location: ~/.timeclip/timeclip.db"