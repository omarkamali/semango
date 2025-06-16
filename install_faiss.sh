#!/bin/bash

echo "Starting FAISS installation..."

# Step 1: Clone the FAISS repository
echo "Cloning FAISS repository..."
git clone https://github.com/blevesearch/faiss.git
if [ $? -ne 0 ]; then
    echo "Error: Failed to clone FAISS repository."
    exit 1
fi

# Step 2: Navigate into the repository directory
echo "Navigating into FAISS directory..."
cd faiss
if [ $? -ne 0 ]; then
    echo "Error: Failed to change directory to faiss."
    exit 1
fi

# Step 3: Update package lists and install SWIG
echo "Updating package lists and installing SWIG..."
if command -v sudo >/dev/null 2>&1; then
    sudo apt-get update && sudo apt-get install -y swig
else
    apt-get update && apt-get install -y swig
fi
if [ $? -ne 0 ]; then
    echo "Error: Failed to install SWIG."
    exit 1
fi

# Step 4: Configure the FAISS build with CMake
echo "Configuring FAISS build with CMake..."
cmake -B build -DFAISS_ENABLE_GPU=OFF -DFAISS_ENABLE_C_API=ON -DBUILD_SHARED_LIBS=ON -DFAISS_ENABLE_PYTHON=OFF -DBUILD_TESTING=OFF -DFAISS_OPT_LEVEL=generic .
if [ $? -ne 0 ]; then
    echo "Error: Failed to configure FAISS with CMake."
    exit 1
fi

# Step 5: Compile the FAISS project
echo "Compiling FAISS project..."
make -C build
if [ $? -ne 0 ]; then
    echo "Error: Failed to compile FAISS."
    exit 1
fi

# Step 6: Copy the compiled FAISS C API library to the project's libs directory
echo "Copying libfaiss_c.so to project libs directory..."
cp build/c_api/libfaiss_c.so ../libs/libfaiss_c.so
if [ $? -ne 0 ]; then
    echo "Error: Failed to copy libfaiss_c.so."
    exit 1
fi

# Step 7: Install the Go module for FAISS
echo "Installing Go module for FAISS..."
go get github.com/blevesearch/go-faiss
if [ $? -ne 0 ]; then
    echo "Error: Failed to install Go FAISS module."
    exit 1
fi

echo "FAISS installation completed successfully!" 