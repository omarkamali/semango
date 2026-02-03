package storage

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/document"
	"github.com/blevesearch/bleve/v2/search"
)

// BleveIndex wraps a Bleve index instance.
type BleveIndex struct {
	idx bleve.Index
}

// OpenOrCreateBleveIndex opens or creates a Bleve index at the given path.
func OpenOrCreateBleveIndex(path string) (*BleveIndex, error) {
	idx, err := bleve.Open(path)
	if err == bleve.ErrorIndexPathDoesNotExist {
		mapping := bleve.NewIndexMapping()
		// Store path to allow reconciliation and stats
		pathFieldMapping := bleve.NewTextFieldMapping()
		pathFieldMapping.Store = true
		mapping.DefaultMapping.AddFieldMappingsAt("path", pathFieldMapping)

		idx, err = bleve.New(path, mapping)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return &BleveIndex{idx: idx}, nil
}

// IndexDocument indexes a document by ID, text, and path.
func (b *BleveIndex) IndexDocument(id, text, path string, meta map[string]string) error {
	doc := map[string]interface{}{
		"text": text,
		"meta": meta,
		"path": path,
	}
	return b.idx.Index(id, doc)
}

// SearchText performs a simple match search on the text field.
func (b *BleveIndex) SearchText(query string, size int) ([]*search.DocumentMatch, error) {
	q := bleve.NewMatchQuery(query)
	sreq := bleve.NewSearchRequestOptions(q, size, 0, false)
	sres, err := b.idx.Search(sreq)
	if err != nil {
		return nil, err
	}
	return sres.Hits, nil
}

// Close closes the Bleve index.
func (b *BleveIndex) Close() error {
	return b.idx.Close()
}

// GetDocument fetches a document by ID from the index.
func (b *BleveIndex) GetDocument(id string) (*document.Document, error) {
	doc, err := b.idx.Document(id)
	if err != nil {
		return nil, err
	}
	d, ok := doc.(*document.Document)
	if !ok {
		return nil, nil
	}
	return d, nil
}

// DocCount returns the number of documents in the index.
func (b *BleveIndex) DocCount() (uint64, error) {
	return b.idx.DocCount()
}

// Delete removes a document by ID.
func (b *BleveIndex) Delete(id string) error {
	return b.idx.Delete(id)
}

// GetAllDocs returns a map of all document IDs to their paths.
func (b *BleveIndex) GetAllDocs() (map[string]string, error) {
	q := bleve.NewMatchAllQuery()
	sreq := bleve.NewSearchRequest(q)
	sreq.Fields = []string{"path"}
	sreq.Size = 1000000 // Limit for now, could be improved with pagination
	sres, err := b.idx.Search(sreq)
	if err != nil {
		return nil, err
	}
	results := make(map[string]string)
	for _, h := range sres.Hits {
		if path, ok := h.Fields["path"].(string); ok {
			results[h.ID] = path
		}
	}
	return results, nil
}
