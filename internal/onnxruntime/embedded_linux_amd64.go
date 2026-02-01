//go:build linux && amd64

package onnxruntime

import _ "embed"

const embeddedLibName = "libonnxruntime.so.1.22.0"

//go:embed embedded/libonnxruntime.so.1.22.0
var embeddedLibData []byte
