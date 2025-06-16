#!/bin/bash

echo "Starting ONNX Runtime installation..."

# Step 1: Download the pre-built ONNX Runtime library for Linux ARM64
echo "Downloading ONNX Runtime pre-built library..."
ONNX_VERSION="1.22.0"
ONNX_URL="https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/onnxruntime-linux-aarch64-${ONNX_VERSION}.tgz"

curl -L -o onnxruntime-linux-aarch64.tgz "$ONNX_URL"
if [ $? -ne 0 ]; then
    echo "Error: Failed to download ONNX Runtime library."
    exit 1
fi

# Step 2: Extract the downloaded archive
echo "Extracting ONNX Runtime library..."
tar -xzf onnxruntime-linux-aarch64.tgz
if [ $? -ne 0 ]; then
    echo "Error: Failed to extract ONNX Runtime library."
    exit 1
fi

# Step 3: Copy the shared library to the project's libs directory
echo "Copying libonnxruntime.so to project libs directory..."
ONNX_DIR="onnxruntime-linux-aarch64-${ONNX_VERSION}"
cp "${ONNX_DIR}/lib/libonnxruntime.so"* libs/
if [ $? -ne 0 ]; then
    echo "Error: Failed to copy libonnxruntime.so."
    exit 1
fi

cd libs
ln -sf libonnxruntime.so onnxruntime.so
cd ..

# Step 4: Copy header files for development (optional but useful)
echo "Copying ONNX Runtime header files..."
mkdir -p include/onnxruntime
cp "${ONNX_DIR}/include/"* include/onnxruntime/
if [ $? -ne 0 ]; then
    echo "Warning: Failed to copy header files (non-critical)."
fi

# Step 5: Install the Go module for ONNX Runtime
echo "Installing Go module for ONNX Runtime..."
go get github.com/yalue/onnxruntime_go
if [ $? -ne 0 ]; then
    echo "Error: Failed to install Go ONNX Runtime module."
    exit 1
fi

# Step 6: Clean up downloaded files
echo "Cleaning up temporary files..."
rm -f onnxruntime-linux-aarch64.tgz
rm -rf "${ONNX_DIR}"

echo "ONNX Runtime installation completed successfully!"
echo "Installed library: $(ls -la libs/libonnxruntime.so*)" 