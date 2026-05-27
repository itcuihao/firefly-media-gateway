#!/bin/bash

# Ensure the script runs relative to the project root
CDPATH="" cd -- "$(dirname -- "$0")/.." || exit 1

# Check if .env file exists
if [ ! -f .env ]; then
  echo "❌ Error: .env file not found!"
  echo "Please copy .env.example to .env and configure your database and token first."
  exit 1
fi

echo "🚀 Loading environment variables from .env..."
set -a
source .env
set +a

# Verify Database URL is set
if [ -z "$DATABASE_URL" ]; then
  echo "❌ Error: DATABASE_URL is not set in .env!"
  exit 1
fi

echo "✨ Starting Firefly Media Gateway server..."
go run ./cmd/server
