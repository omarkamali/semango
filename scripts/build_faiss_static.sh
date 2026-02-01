#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
FAISS_DIR="$ROOT_DIR/faiss"
INSTALL_DIR="$ROOT_DIR/libs-static"
INCLUDE_DIR="$ROOT_DIR/include/faiss"

mkdir -p "$INSTALL_DIR"
mkdir -p "$INCLUDE_DIR"

if [ ! -d "$FAISS_DIR" ]; then
    echo "FAISS source not found at $FAISS_DIR. Cloning..."
    git clone --branch bleve --single-branch https://github.com/blevesearch/faiss.git "$FAISS_DIR"
    cd "$FAISS_DIR"
    git checkout b3d4e00a69425b95e0b283da7801efc9f66b580d
    cd ..
fi

cd "$FAISS_DIR"

# Set include path to repo root so <faiss/Header.h> works
export CXXFLAGS="-I$PWD"

CMAKE_OPTS="-DFAISS_ENABLE_GPU=OFF -DFAISS_ENABLE_C_API=ON -DBUILD_SHARED_LIBS=OFF -DFAISS_ENABLE_PYTHON=OFF -DBUILD_TESTING=OFF -DFAISS_OPT_LEVEL=generic"

if [ "$(uname -s)" == "Darwin" ]; then
    CMAKE_OPTS="$CMAKE_OPTS -DOpenMP_ROOT=$(brew --prefix libomp)"
fi

cmake -B build $CMAKE_OPTS -DCMAKE_CXX_FLAGS="-I$PWD" .
make -C build -j$(nproc 2>/dev/null || sysctl -n hw.ncpu) faiss faiss_c

find build -name "libfaiss_c.a" -exec cp {} "$INSTALL_DIR/" \;
find build -name "libfaiss.a" -exec cp {} "$INSTALL_DIR/" \;

# Copy headers for CGO
cp -r c_api "$INCLUDE_DIR/"

echo "FAISS static libraries built successfully in $INSTALL_DIR"
