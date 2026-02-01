//go:build darwin && amd64

package onnxruntime

import _ "embed"

const embeddedLibName = "libonnxruntime.1.22.0.dylib"

//go:embed embedded/libonnxruntime.1.22.0.dylib
var embeddedLibData []byte
