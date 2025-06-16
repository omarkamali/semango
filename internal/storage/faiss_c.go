//go:build linux && amd64
// +build linux,amd64

package storage

/*
#cgo CXXFLAGS: -std=c++11
#cgo CPPFLAGS: -I/usr/local/include -I/usr/include
#cgo LDFLAGS: -lfaiss_c -L/usr/local/lib -L/usr/lib
// Include FAISS C API headers
// #include <faiss/c_api/Index_c.h>
// #include <faiss/c_api/IndexIVF_c.h>
// #include <faiss/c_api/IndexIVF_c_ex.h>
// #include <faiss/c_api/Index_c_ex.h>
// #include <faiss/c_api/impl/AuxIndexStructures_c.h>
// #include <faiss/c_api/index_factory_c.h>
// #include <faiss/c_api/MetaIndexes_c.h>
*/
import "C"

// This file provides CGO directives for building with FAISS.
// Go functions that directly wrap C calls can also be placed here.
