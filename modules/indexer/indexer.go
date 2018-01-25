// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"fmt"
	"strconv"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/mapping"
	"github.com/blevesearch/bleve/search/query"
)

// indexerID a bleve-compatible unique identifier for an integer id
func indexerID(id int64) string {
	return strconv.FormatInt(id, 36)
}

// idOfIndexerID the integer id associated with an indexer id
func idOfIndexerID(indexerID string) (int64, error) {
	id, err := strconv.ParseInt(indexerID, 36, 64)
	if err != nil {
		return 0, fmt.Errorf("Unexpected indexer ID %s: %v", indexerID, err)
	}
	return id, nil
}

// numericEqualityQuery a numeric equality query for the given value and field
func numericEqualityQuery(value int64, field string) *query.NumericRangeQuery {
	f := float64(value)
	tru := true
	q := bleve.NewNumericRangeInclusiveQuery(&f, &f, &tru, &tru)
	q.SetField(field)
	return q
}

func newMatchPhraseQuery(matchPhrase, field, analyzer string) *query.MatchPhraseQuery {
	q := bleve.NewMatchPhraseQuery(matchPhrase)
	q.FieldVal = field
	q.Analyzer = analyzer
	return q
}

const unicodeNormalizeName = "unicodeNormalize"

func addUnicodeNormalizeTokenFilter(m *mapping.IndexMappingImpl) error {
	return m.AddCustomTokenFilter(unicodeNormalizeName, map[string]interface{}{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	})
}

// Update represents an update to an indexer
type Update interface {
	addToBatch(batch *bleve.Batch) error
}

const maxBatchSize = 16

// Batch batch of indexer updates that automatically flushes once it
// reaches a certain size
type Batch struct {
	batch *bleve.Batch
	index bleve.Index
}

// Add add update to batch, possibly flushing
func (batch *Batch) Add(update Update) error {
	if err := update.addToBatch(batch.batch); err != nil {
		return err
	}
	return batch.flushIfFull()
}

func (batch *Batch) flushIfFull() error {
	if batch.batch.Size() >= maxBatchSize {
		return batch.Flush()
	}
	return nil
}

// Flush manually flush the batch, regardless of its size
func (batch *Batch) Flush() error {
	if err := batch.index.Batch(batch.batch); err != nil {
		return err
	}
	batch.batch.Reset()
	return nil
}
