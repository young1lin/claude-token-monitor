#!/bin/bash

echo "Building claude-token-monitor for all platforms..."

# Create bin directories
mkdir -p bin
mkdir -p statusline-plugin/bin

# Build monitor for all platforms
echo "Building monitor..."
for GOOS in windows darwin linux; do
  for GOARCH in amd64 arm64; do
    EXT=""
    if [ "$GOOS" = "windows" ]; then
      EXT=".exe"
    fi
    echo "  Building monitor for $GOOS-$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o bin/claude-token-monitor-$GOOS-$GOARCH$EXT -ldflags="-s -w" ./cmd/monitor
  done
done

# Build statusline for all platforms
echo "Building statusline..."
for GOOS in windows darwin linux; do
  for GOARCH in amd64 arm64; do
    EXT=""
    if [ "$GOOS" = "windows" ]; then
      EXT=".exe"
    fi
    echo "  Building statusline for $GOOS-$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o statusline-plugin/bin/statusline-$GOOS-$GOARCH$EXT -ldflags="-s -w" ./cmd/statusline
  done
done

echo "Build complete!"
echo ""
echo "Monitor binaries:"
ls -lh bin/ | grep claude-token-monitor
echo ""
echo "Statusline binaries:"
ls -lh statusline-plugin/bin/ | grep statusline
