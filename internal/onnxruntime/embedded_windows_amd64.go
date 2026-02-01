//go:build windows && amd64

package onnxruntime

import _ "embed"

const embeddedLibName = "onnxruntime.dll"

//go:embed embedded/onnxruntime.dll
var embeddedLibData []byte
