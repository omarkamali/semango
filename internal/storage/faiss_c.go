package storage

/*
#cgo CXXFLAGS: -std=c++11
#cgo CPPFLAGS: -I/usr/local/include/faiss/c_api -I/usr/include/faiss/c_api
#cgo LDFLAGS: -lfaiss_c -L/usr/local/lib -L/usr/lib
// It's often good practice to include specific headers needed, e.g.:
// #include "faiss/c_api/Index_c.h"
// #include "faiss/c_api/MetaIndexes_c.h"
// #include "faiss/c_api/index_io_c.h"
// #include "faiss/c_api/AutoTune_c.h"
*/
import "C"

// This file provides CGO directives for building with FAISS.
// Go functions that directly wrap C calls can also be placed here.
