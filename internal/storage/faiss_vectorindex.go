//go:build cgo && linux && amd64
// +build cgo,linux,amd64

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
)

// FaissVectorIndex adapts FaissIndex to the VectorIndex interface.
// It hashes string IDs to int64 labels deterministically using FNV-1a.
// The mapping does not guarantee collision-freeness but is sufficient for
// small/medium corpora and demo purposes. For production use, a persistent
// hashmap (e.g. BoltDB) is advised.

type FaissVectorIndex struct {
	fi        *FaissIndex
	indexPath string
	hashBuf   [8]byte // scratch buffer for hashing
	idToLabel map[string]int64
	labelToID map[int64]string
	nextLabel int64
}

// NewFaissVectorIndex opens or creates the FAISS index at the given path with
// the provided dimension and metric.
func NewFaissVectorIndex(ctx context.Context, indexPath string, dim int, metric int) (*FaissVectorIndex, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return nil, err
	}

	fi, err := NewFaissIndex(ctx, indexPath, dim, metric)
	if err != nil {
		return nil, err
	}

	fvi := &FaissVectorIndex{
		fi:        fi,
		indexPath: indexPath,
		idToLabel: map[string]int64{},
		labelToID: map[int64]string{},
		nextLabel: 1,
	}

	// Load mapping file if exists
	mapPath := indexPath + ".ids.json"
	if data, err := os.ReadFile(mapPath); err == nil {
		_ = json.Unmarshal(data, &fvi.idToLabel)
		for k, v := range fvi.idToLabel {
			fvi.labelToID[v] = k
			if v >= fvi.nextLabel {
				fvi.nextLabel = v + 1
			}
		}
	}

	return fvi, nil
}

func (f *FaissVectorIndex) hashID(id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	sum := h.Sum64()
	// Interpret as signed int64
	return int64(sum)
}

// Upsert inserts or replaces a vector for the given ID.
func (f *FaissVectorIndex) Upsert(ctx context.Context, id string, vector []float32) error {
	label, ok := f.idToLabel[id]
	if !ok {
		label = f.nextLabel
		f.nextLabel++
		f.idToLabel[id] = label
		f.labelToID[label] = id
	}
	vectors := [][]float32{vector}
	ids := []int64{label}

	// For simplicity, we attempt Add; FAISS IDMap will overwrite existing IDs.
	if err := f.fi.Add(ctx, vectors, ids); err != nil {
		return err
	}
	// Persist to disk after each upsert for now; could batch in future.
	f.persistMap()
	return f.fi.Save(ctx)
}

func (f *FaissVectorIndex) Search(ctx context.Context, query []float32, topK int) ([]VectorResult, error) {
	distances, labels, err := f.fi.Search(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	results := make([]VectorResult, len(labels))
	for i, l := range labels {
		id, ok := f.labelToID[l]
		if !ok {
			id = fmt.Sprintf("%d", l)
		}
		results[i] = VectorResult{ID: id, Score: distances[i]}
	}
	return results, nil
}

func (f *FaissVectorIndex) Dimension() int {
	return f.fi.Dim()
}

func (f *FaissVectorIndex) Close() error {
	// Save index before closing to persist vectors.
	_ = f.fi.Save(context.Background())
	f.persistMap()
	f.fi.Close(context.Background())
	return nil
}

func (f *FaissVectorIndex) persistMap() {
	mapPath := f.indexPath + ".ids.json"
	data, _ := json.MarshalIndent(f.idToLabel, "", "  ")
	_ = os.WriteFile(mapPath, data, 0o644)
}
