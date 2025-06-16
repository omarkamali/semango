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
		idx, err = bleve.New(path, mapping)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return &BleveIndex{idx: idx}, nil
}

// IndexDocument indexes a document by ID and text.
func (b *BleveIndex) IndexDocument(id, text string, meta map[string]string) error {
	doc := map[string]interface{}{
		"text": text,
		"meta": meta,
	}
	if p, ok := meta["path"]; ok {
		doc["path"] = p
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
