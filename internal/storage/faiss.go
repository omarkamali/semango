package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/blevesearch/go-faiss"
	"github.com/omneity-labs/semango/internal/util/log"
)

// FaissIndex represents a FAISS vector index.
// It wraps the blevesearch/go-faiss Index interface and provides
// additional methods for Semango-specific functionality.
type FaissIndex struct {
	index faiss.Index
	dim   int    // Dimension of the vectors
	path  string // Path to the index file on disk
}

// NewFaissIndex creates or loads a FAISS index.
// If an index file exists at the given path, it's loaded.
// Otherwise, a new index is created with the specified dimension and metric.
// The metric argument should be one of the faiss.Metric... constants (e.g., faiss.MetricL2, faiss.MetricInnerProduct).
func NewFaissIndex(ctx context.Context, path string, dim int, metric int) (*FaissIndex, error) {
	logger := log.FromContext(ctx)

	// Check if index file exists
	if _, err := os.Stat(path); err == nil {
		// File exists, try to load it
		logger.Info("FAISS index file found, attempting to load.", "path", path)
		// The spec requires IO_FLAG_MMAP
		idx, loadErr := faiss.ReadIndex(path, faiss.IOFlagMmap)
		if loadErr == nil {
			// Verify dimension and metric type if possible, though the lib might not expose metric type easily after load.
			// For now, assume it's compatible if loaded successfully. D() method gives dimension.
			if idx.D() != dim {
				logger.Error("Loaded FAISS index dimension mismatch.", "path", path, "expected_dim", dim, "actual_dim", idx.D())
				idx.Close() // Free resources if not usable
				return nil, fmt.Errorf("loaded FAISS index dimension mismatch: expected %d, got %d", dim, idx.D())
			}
			// We can't easily get the metric type from the loaded index via go-faiss to compare with `metric` param.
			// We'll assume the user provides the correct metric for existing indexes or relies on the stored one.
			logger.Info("Successfully loaded FAISS index from disk.", "path", path, "dimension", idx.D(), "total_vectors", idx.Ntotal())
			return &FaissIndex{
				index: idx,
				dim:   idx.D(), // Use dimension from loaded index
				path:  path,
			}, nil
		}
		logger.Warn("Found FAISS index file, but failed to load. Will attempt to create a new one.", "path", path, "error", loadErr)
		// If loading fails, proceed to create a new one (could be due to corruption or incompatibility)
	} else if !os.IsNotExist(err) {
		// Some other error occurred with os.Stat (e.g., permission issue)
		logger.Error("Error checking for FAISS index file", "path", path, "error", err)
		return nil, fmt.Errorf("error checking for FAISS index file at %s: %w", path, err)
	}

	// If not, create a new one:
	logger.Info("Creating new FAISS index.", "path", path, "dimension", dim, "metric_code", metric)

	var idx faiss.Index
	var err error

	// The faiss.NewIndexFlat function takes the metric type as an int.
	// faiss.MetricInnerProduct and faiss.MetricL2 are such int constants.
	idx, err = faiss.NewIndexFlat(dim, metric)

	if err != nil {
		logger.Error("Failed to create new FAISS index", "error", err, "path", path, "dimension", dim, "metric_code", metric)
		return nil, fmt.Errorf("faiss.NewIndexFlat (metric %d): %w", metric, err)
	}

	logger.Info("Successfully created new FAISS index", "path", path, "dimension", dim, "metric_code", metric)
	return &FaissIndex{
		index: idx,
		dim:   dim,
		path:  path,
	}, nil
}

// Add vectors to the index.
func (fi *FaissIndex) Add(ctx context.Context, vectors [][]float32, ids []int64) error {
	logger := log.FromContext(ctx)
	if len(vectors) == 0 {
		logger.Debug("No vectors to add to FAISS index")
		return nil
	}
	if len(vectors[0]) != fi.dim {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", fi.dim, len(vectors[0]))
	}

	flattenedVectors := make([]float32, 0, len(vectors)*fi.dim)
	for _, vec := range vectors {
		flattenedVectors = append(flattenedVectors, vec...)
	}

	if ids != nil && len(ids) != len(vectors) {
		return fmt.Errorf("number of IDs does not match number of vectors: %d IDs, %d vectors", len(ids), len(vectors))
	}

	var err error
	if ids != nil {
		err = fi.index.AddWithIDs(flattenedVectors, ids)
	} else {
		err = fi.index.Add(flattenedVectors)
	}

	if err != nil {
		logger.Error("Failed to add vectors to FAISS index", "error", err, "num_vectors", len(vectors))
		return fmt.Errorf("FaissIndex.Add: %w", err)
	}
	logger.Debug("Successfully added vectors to FAISS index", "num_vectors", len(vectors), "total_vectors", fi.index.Ntotal())
	return nil
}

// Search for k-nearest neighbors.
// Returns distances, labels (IDs), and an error if any.
func (fi *FaissIndex) Search(ctx context.Context, queryVector []float32, k int) ([]float32, []int64, error) {
	logger := log.FromContext(ctx)
	if len(queryVector) != fi.dim {
		return nil, nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", fi.dim, len(queryVector))
	}

	distances, labels, err := fi.index.Search(queryVector, int64(k))
	if err != nil {
		logger.Error("Failed to search FAISS index", "error", err, "query_dim", len(queryVector), "k", k)
		return nil, nil, fmt.Errorf("FaissIndex.Search: %w", err)
	}
	logger.Debug("Successfully searched FAISS index", "k", k, "results_count", len(labels))
	return distances, labels, nil
}

// Save the index to disk.
func (fi *FaissIndex) Save(ctx context.Context) error {
	logger := log.FromContext(ctx)
	err := faiss.WriteIndex(fi.index, fi.path)
	if err != nil {
		logger.Error("Failed to save FAISS index to disk", "error", err, "path", fi.path)
		return fmt.Errorf("faiss.WriteIndex: %w", err)
	}
	logger.Info("Successfully saved FAISS index to disk", "path", fi.path)
	return nil
}

// Close the index and release resources.
func (fi *FaissIndex) Close(ctx context.Context) {
	logger := log.FromContext(ctx)
	if fi.index != nil {
		fi.index.Close() // blevesearch/go-faiss uses Close() to free memory
		logger.Info("FAISS index closed", "path", fi.path)
	}
}

// Ntotal returns the total number of vectors in the index.
func (fi *FaissIndex) Ntotal(ctx context.Context) int64 {
	if fi.index == nil {
		return 0
	}
	return fi.index.Ntotal()
}

// Dim returns the dimension of the vectors in the index.
func (fi *FaissIndex) Dim() int {
	return fi.dim
}
