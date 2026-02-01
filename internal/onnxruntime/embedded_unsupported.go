//go:build !(linux && (amd64 || arm64)) && !(darwin && (amd64 || arm64)) && !(windows && amd64)

package onnxruntime

const embeddedLibName = ""

var embeddedLibData []byte
