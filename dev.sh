#!/bin/bash
set -e

PLATFORM=${1:-mac}

echo "Starting Blue Banded Bee development environment (platform: $PLATFORM)..."

# Check if Docker is running
if ! docker ps >/dev/null 2>&1; then
    echo "Error: Docker is not running. Please start Docker first."
    echo "Download: https://docs.docker.com/desktop/"
    exit 1
fi

# Check if Supabase CLI is installed
if ! command -v supabase >/dev/null 2>&1; then
    echo "Error: Supabase CLI is not installed."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "Install with: brew install supabase/tap/supabase"
    else
        echo "Install with: npm install -g supabase"
    fi
    echo "Or download from: https://supabase.com/docs/guides/cli"
    exit 1
fi

# Configure Air for platform
if [ "$PLATFORM" = "pc" ]; then
    echo "Configuring Air for Windows..."
    sed -i.bak \
        -e 's/^cmd = "go build -o \.\/tmp\/main "/# cmd = "go build -o .\/tmp\/main "/' \
        -e 's/^bin = "tmp\/main"/# bin = "tmp\/main"/' \
        -e 's/^# cmd = "go build -o \.\/tmp\/main\.exe/cmd = "go build -o .\/tmp\/main.exe/' \
        -e 's/^# bin = "tmp\/main\.exe"/bin = "tmp\/main.exe"/' \
        .air.toml
else
    echo "Configuring Air for Mac/Linux..."
    sed -i.bak \
        -e 's/^cmd = "go build -o \.\/tmp\/main\.exe/# cmd = "go build -o .\/tmp\/main.exe/' \
        -e 's/^bin = "tmp\/main\.exe"/# bin = "tmp\/main.exe"/' \
        -e 's/^# cmd = "go build -o \.\/tmp\/main "/cmd = "go build -o .\/tmp\/main "/' \
        -e 's/^# bin = "tmp\/main"/bin = "tmp\/main"/' \
        .air.toml
fi

# Start Supabase (will be no-op if already running)
echo "Starting local Supabase..."
supabase start

# Start Air with hot reloading
echo "Starting development server with hot reloading..."
air