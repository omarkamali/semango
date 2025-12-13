//go:build !cgo || !linux || !amd64
// +build !cgo linux,!amd64 !linux

package storage

import (
    "context"
    "errors"
)

var errFaissUnavailable = errors.New("faiss support requires CGO on linux/amd64")

// FaissIndex is a stub used when CGO or the required platform is unavailable.
type FaissIndex struct{}

func NewFaissIndex(_ context.Context, _ string, _ int, _ int) (*FaissIndex, error) {
    return nil, errFaissUnavailable
}

func (fi *FaissIndex) Add(_ context.Context, _ [][]float32, _ []int64) error {
    return errFaissUnavailable
}

func (fi *FaissIndex) Search(_ context.Context, _ []float32, _ int) ([]float32, []int64, error) {
    return nil, nil, errFaissUnavailable
}

func (fi *FaissIndex) Save(_ context.Context) error {
    return errFaissUnavailable
}

func (fi *FaissIndex) Close(_ context.Context) {}

func (fi *FaissIndex) Ntotal(_ context.Context) int64 { return 0 }

func (fi *FaissIndex) Dim() int { return 0 }

// FaissVectorIndex is a stub used when CGO or the required platform is unavailable.
type FaissVectorIndex struct{}

func NewFaissVectorIndex(_ context.Context, _ string, _ int, _ int) (*FaissVectorIndex, error) {
    return nil, errFaissUnavailable
}

func (f *FaissVectorIndex) Upsert(_ context.Context, _ string, _ []float32) error {
    return errFaissUnavailable
}

func (f *FaissVectorIndex) Search(_ context.Context, _ []float32, _ int) ([]VectorResult, error) {
    return nil, errFaissUnavailable
}

func (f *FaissVectorIndex) Dimension() int { return 0 }

func (f *FaissVectorIndex) Close() error { return errFaissUnavailable }
