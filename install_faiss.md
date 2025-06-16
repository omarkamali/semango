# FAISS Installation Guide

This guide provides instructions for installing FAISS (Facebook AI Similarity Search) and its Go module on a Linux system. You can choose to follow the manual steps or use the provided automated script.

## Manual Installation Steps

To install FAISS and its Go module manually, follow these steps in your terminal:

1.  **Clone the FAISS repository:**
    ```bash
    git clone https://github.com/blevesearch/faiss.git
    ```

2.  **Navigate into the cloned directory:**
    ```bash
    cd faiss
    ```

3.  **Update package lists and install SWIG:**
    This command checks if `sudo` is available and uses it if possible, otherwise it attempts to install without `sudo`.
    ```bash
    if command -v sudo >/dev/null 2>&1; then
        sudo apt-get update && sudo apt-get install -y swig
    else
        apt-get update && apt-get install -y swig
    fi
    ```

4.  **Configure the FAISS build with CMake:**
    This step configures the build to disable GPU support (`-DFAISS_ENABLE_GPU=OFF`), enable the C API (`-DFAISS_ENABLE_C_API=ON`), build shared libraries (`-DBUILD_SHARED_LIBS=ON`), disable Python bindings (`-DFAISS_ENABLE_PYTHON=OFF` to avoid dependency issues with Python), and disable building tests (`-DBUILD_TESTING=OFF` to avoid test compilation failures).
    ```bash
    cmake -B build -DFAISS_ENABLE_GPU=OFF -DFAISS_ENABLE_C_API=ON -DBUILD_SHARED_LIBS=ON -DFAISS_ENABLE_PYTHON=OFF -DBUILD_TESTING=OFF .
    ```

5.  **Compile the FAISS project:**
    ```bash
    make -C build
    ```

6.  **Install the compiled FAISS libraries:**
    This command installs the compiled shared libraries (e.g., `libfaiss.so` and `libfaiss_c.so`) and header files to their standard system locations (typically `/usr/local/lib` and `/usr/local/include/faiss`).
    ```bash
    if command -v sudo >/dev/null 2>&1; then
        sudo make -C build install
    else
        make -C build install
    fi
    ```

7.  **Install the Go module for FAISS:**
    ```bash
    go get github.com/blevesearch/go-faiss
    ```

## Automated Installation Script

For a fully automated installation, you can use the `install_faiss.sh` script located in the root of this project. This script performs all the manual steps listed above.

To use the script:

1.  **Make the script executable:**
    ```bash
    chmod +x install_faiss.sh
    ```

2.  **Run the script:**
    ```bash
    ./install_faiss.sh
    ```

The script will output its progress and any errors encountered during the installation process. 