#!/bin/bash

# Build script for the DEXTR Price Feeder Docker container
# Supports multi-architecture builds for amd64 and arm64

set -e

# Configuration
IMAGE_NAME="${1:-dextr-price-feeder}"
TAG="${2:-latest}"
BUILD_MODE="${3:-auto}"  # auto, amd64, arm64

# Function to show usage
show_usage() {
    echo "Usage: $0 [IMAGE_NAME] [TAG] [BUILD_MODE]"
    echo ""
    echo "Arguments:"
    echo "  IMAGE_NAME     Docker image name (default: dextr-price-feeder)"
    echo "  TAG            Docker image tag (default: latest)"
    echo "  BUILD_MODE     Build mode (default: auto)"
    echo ""
    echo "Build modes:"
    echo "  auto           Build for current architecture (default)"
    echo "  amd64          Build for AMD64/Intel architecture"
    echo "  arm64          Build for ARM64 architecture"
    echo ""
    echo "Examples:"
    echo "  $0                          # Build dextr-price-feeder:latest for current arch"
    echo "  $0 my-app                  # Build my-app:latest for current arch"
    echo "  $0 my-app v1.0.0           # Build my-app:v1.0.0 for current arch"
    echo "  $0 my-app latest amd64     # Build my-app:latest for AMD64"
    echo "  $0 my-app latest arm64     # Build my-app:latest for ARM64"
}

# Check if help is requested
if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    show_usage
    exit 0
fi

echo "🔨 Building Docker image: ${IMAGE_NAME}:${TAG}"
echo "📦 Build mode: ${BUILD_MODE}"

# Detect current architecture
CURRENT_ARCH=$(uname -m)
case $CURRENT_ARCH in
    x86_64) PLATFORM="linux/amd64" ;;
    arm64|aarch64) PLATFORM="linux/arm64" ;;
    *) PLATFORM="linux/amd64" ;;
esac

echo "🖥️  Current architecture: ${CURRENT_ARCH} (${PLATFORM})"

# Determine target platform based on build mode
case $BUILD_MODE in
    "auto")
        TARGET_PLATFORM="$PLATFORM"
        echo "🔧 Building for current architecture: ${TARGET_PLATFORM}"
        ;;
    "amd64")
        TARGET_PLATFORM="linux/amd64"
        echo "🔧 Building for AMD64 architecture: ${TARGET_PLATFORM}"
        ;;
    "arm64")
        TARGET_PLATFORM="linux/arm64"
        echo "🔧 Building for ARM64 architecture: ${TARGET_PLATFORM}"
        ;;
    *)
        echo "❌ Invalid build mode: ${BUILD_MODE}"
        echo ""
        show_usage
        exit 1
        ;;
esac

# Build the image
# Note: Build context must be the parent directory (catalystlabs/) to access both core/ and dextr-avs/
ORIGINAL_DIR=$(pwd)
cd ../.. || exit 1
BUILD_CONTEXT=$(pwd)
echo "📁 Build context: ${BUILD_CONTEXT}"

if [[ "$BUILD_MODE" == "auto" ]]; then
    # Use regular docker build for current architecture (fastest)
    docker build -f dextr-avs/price-feeder/Dockerfile -t "${IMAGE_NAME}:${TAG}" .
    BUILD_RESULT=$?
    cd "$ORIGINAL_DIR" || exit 1
    
    if [ $BUILD_RESULT -eq 0 ]; then
        echo "✅ Successfully built ${IMAGE_NAME}:${TAG} for ${TARGET_PLATFORM}"
    else
        echo "❌ Build failed"
        exit 1
    fi
else
    # Use buildx for cross-platform builds
    echo "🛠️  Using buildx for cross-platform build..."
    
    # Ensure buildx builder exists
    if ! docker buildx inspect dextr-builder >/dev/null 2>&1; then
        echo "🛠️  Creating new buildx builder..."
        docker buildx create --name dextr-builder --use
    else
        docker buildx use dextr-builder
    fi
    
    docker buildx build \
        --platform "${TARGET_PLATFORM}" \
        --tag "${IMAGE_NAME}:${TAG}" \
        --load \
        -f dextr-avs/price-feeder/Dockerfile \
        .
    BUILD_RESULT=$?
    cd "$ORIGINAL_DIR" || exit 1
    
    if [ $BUILD_RESULT -eq 0 ]; then
        echo "✅ Successfully built and loaded ${IMAGE_NAME}:${TAG} for ${TARGET_PLATFORM}"
    else
        echo "❌ Build failed"
        exit 1
    fi
fi

echo ""
echo "🎉 Build complete! Image: ${IMAGE_NAME}:${TAG}"
echo "📋 Built for: ${TARGET_PLATFORM}"

# Show build method used
if [[ "$BUILD_MODE" == "auto" ]]; then
    echo "🔧 Built using: Regular Docker build (fastest)"
else
    echo "🔧 Built using: Docker buildx (cross-platform)"
fi

echo ""
echo "🚀 To run the built image:"
echo "   docker run -p 8080:8080 -p 9091:9091 ${IMAGE_NAME}:${TAG}"
echo ""
echo "🐳 Or use docker-compose:"
echo "   docker-compose up -d"
