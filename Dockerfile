FROM golang:1.23-bookworm

# Set environment variables
ENV CGO_ENABLED=1
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies for CGO, Tesseract, and CUE
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    tesseract-ocr \
    libleptonica-dev \
    curl \
    cmake \
    libopenblas-dev \
    libgflags-dev \
    libfaiss-dev \
    swig \
    && rm -rf /var/lib/apt/lists/*

# Install CUE CLI
RUN CUE_VERSION="0.9.2" && \
    ARCH=$(dpkg --print-architecture) && \
    curl -fsSL "https://github.com/cue-lang/cue/releases/download/v${CUE_VERSION}/cue_v${CUE_VERSION}_linux_${ARCH}.tar.gz" -o cue.tar.gz && \
    tar -xzf cue.tar.gz -C /usr/local/bin cue && \
    rm cue.tar.gz && \
    cue version

# Set up the working directory
WORKDIR /app

# Copy Go module files and download dependencies
# This layer is cached when go.mod and go.sum don't change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the rest of the application source code
COPY . .

# Expose ports for the application
EXPOSE 8181 50051

# Default command to keep the container running for development or to run the app
# For development, one might override this or connect with an interactive shell.
CMD tail -f /dev/null

# Download and build FAISS C API
ENV FAISS_VERSION=1.8.0
RUN curl -L "https://github.com/facebookresearch/faiss/archive/refs/tags/v${FAISS_VERSION}.tar.gz" -o faiss.tar.gz && \
    tar -xzf faiss.tar.gz && \
    mv faiss-${FAISS_VERSION} /opt/faiss_src && \
    rm faiss.tar.gz && \
    cd /opt/faiss_src && \
    cmake -B build -DFAISS_ENABLE_GPU=OFF -DFAISS_ENABLE_PYTHON=OFF -DFAISS_ENABLE_C_API=ON -DBUILD_SHARED_LIBS=ON -DCMAKE_BUILD_TYPE=Release . && \
    make -C build -j$(nproc) faiss_c && \
    make -C build install && \
    # Copy C API headers for Go bindings
    cp -r c_api /usr/local/include/faiss && \
    # Cleanup build artifacts
    cd / && rm -rf /opt/faiss_src 