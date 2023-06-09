// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/indexer/internal"
	inner_bleve "code.gitea.io/gitea/modules/indexer/internal/bleve"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

const (
	issueIndexerAnalyzer      = "issueIndexer"
	issueIndexerDocType       = "issueIndexerDocType"
	issueIndexerLatestVersion = 2
)

// indexerID a bleve-compatible unique identifier for an integer id
func indexerID(id int64) string {
	return strconv.FormatInt(id, 36)
}

// idOfIndexerID the integer id associated with an indexer id
func idOfIndexerID(indexerID string) (int64, error) {
	id, err := strconv.ParseInt(indexerID, 36, 64)
	if err != nil {
		return 0, fmt.Errorf("Unexpected indexer ID %s: %w", indexerID, err)
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

const maxBatchSize = 16

// BleveIndexerData an update to the issue indexer
type BleveIndexerData IndexerData

// Type returns the document type, for bleve's mapping.Classifier interface.
func (i *BleveIndexerData) Type() string {
	return issueIndexerDocType
}

// generateIssueIndexMapping generates the bleve index mapping for issues
func generateIssueIndexMapping() (mapping.IndexMapping, error) {
	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Store = false
	textFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("Title", textFieldMapping)
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)
	docMapping.AddFieldMappingsAt("Comments", textFieldMapping)

	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return nil, err
	} else if err = mapping.AddCustomAnalyzer(issueIndexerAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, camelcase.Name, lowercase.Name},
	}); err != nil {
		return nil, err
	}

	mapping.DefaultAnalyzer = issueIndexerAnalyzer
	mapping.AddDocumentMapping(issueIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())

	return mapping, nil
}

var _ Indexer = &BleveIndexer{}

// BleveIndexer implements Indexer interface
type BleveIndexer struct {
	inner            *inner_bleve.Indexer
	internal.Indexer // do not composite inner_bleve.Indexer directly to avoid exposing too much
}

// NewBleveIndexer creates a new bleve local indexer
func NewBleveIndexer(indexDir string) *BleveIndexer {
	inner := inner_bleve.NewIndexer(indexDir, issueIndexerLatestVersion, generateIssueIndexMapping)
	return &BleveIndexer{
		Indexer: inner,
		inner:   inner,
	}
}

// Index will save the index data
func (b *BleveIndexer) Index(issues []*IndexerData) error {
	batch := inner_bleve.NewFlushingBatch(b.inner.Indexer, maxBatchSize)
	for _, issue := range issues {
		if err := batch.Index(indexerID(issue.ID), struct {
			RepoID   int64
			Title    string
			Content  string
			Comments []string
		}{
			RepoID:   issue.RepoID,
			Title:    issue.Title,
			Content:  issue.Content,
			Comments: issue.Comments,
		}); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Delete deletes indexes by ids
func (b *BleveIndexer) Delete(ids ...int64) error {
	batch := inner_bleve.NewFlushingBatch(b.inner.Indexer, maxBatchSize)
	for _, id := range ids {
		if err := batch.Delete(indexerID(id)); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *BleveIndexer) Search(ctx context.Context, keyword string, repoIDs []int64, limit, start int) (*SearchResult, error) {
	var repoQueriesP []*query.NumericRangeQuery
	for _, repoID := range repoIDs {
		repoQueriesP = append(repoQueriesP, numericEqualityQuery(repoID, "RepoID"))
	}
	repoQueries := make([]query.Query, len(repoQueriesP))
	for i, v := range repoQueriesP {
		repoQueries[i] = query.Query(v)
	}

	indexerQuery := bleve.NewConjunctionQuery(
		bleve.NewDisjunctionQuery(repoQueries...),
		bleve.NewDisjunctionQuery(
			newMatchPhraseQuery(keyword, "Title", issueIndexerAnalyzer),
			newMatchPhraseQuery(keyword, "Content", issueIndexerAnalyzer),
			newMatchPhraseQuery(keyword, "Comments", issueIndexerAnalyzer),
		))
	search := bleve.NewSearchRequestOptions(indexerQuery, limit, start, false)
	search.SortBy([]string{"-_score"})

	result, err := b.inner.Indexer.SearchInContext(ctx, search)
	if err != nil {
		return nil, err
	}

	ret := SearchResult{
		Hits: make([]Match, 0, len(result.Hits)),
	}
	for _, hit := range result.Hits {
		id, err := idOfIndexerID(hit.ID)
		if err != nil {
			return nil, err
		}
		ret.Hits = append(ret.Hits, Match{
			ID: id,
		})
	}
	return &ret, nil
}
