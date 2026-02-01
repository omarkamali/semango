package onnxruntime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	embedOnce     sync.Once
	embedPath     string
	embedPathErr  error
)

// EnsureSharedLibrary writes the embedded onnxruntime shared library to disk
// and returns the path to the extracted file.
func EnsureSharedLibrary() (string, error) {
	embedOnce.Do(func() {
		if len(embeddedLibData) == 0 || embeddedLibName == "" {
			embedPathErr = errors.New("embedded onnxruntime library not available for this platform")
			return
		}

		cacheRoot, err := os.UserCacheDir()
		if err != nil || cacheRoot == "" {
			cacheRoot = os.TempDir()
		}

		targetDir := filepath.Join(cacheRoot, "semango", "onnxruntime", runtime.GOOS, runtime.GOARCH)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			embedPathErr = fmt.Errorf("create cache dir: %w", err)
			return
		}

		targetPath := filepath.Join(targetDir, embeddedLibName)
		if info, err := os.Stat(targetPath); err == nil {
			if info.Size() == int64(len(embeddedLibData)) {
				embedPath = targetPath
				return
			}
		}

		if err := os.WriteFile(targetPath, embeddedLibData, 0o755); err != nil {
			embedPathErr = fmt.Errorf("write embedded onnxruntime library: %w", err)
			return
		}

		embedPath = targetPath
	})

	return embedPath, embedPathErr
}
