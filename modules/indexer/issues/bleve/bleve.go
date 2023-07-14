// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"context"

	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_bleve "code.gitea.io/gitea/modules/indexer/internal/bleve"
	"code.gitea.io/gitea/modules/indexer/issues/internal"

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
	issueIndexerLatestVersion = 3
)

const unicodeNormalizeName = "unicodeNormalize"

func addUnicodeNormalizeTokenFilter(m *mapping.IndexMappingImpl) error {
	return m.AddCustomTokenFilter(unicodeNormalizeName, map[string]any{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	})
}

const maxBatchSize = 16

// IndexerData an update to the issue indexer
type IndexerData internal.IndexerData

// Type returns the document type, for bleve's mapping.Classifier interface.
func (i *IndexerData) Type() string {
	return issueIndexerDocType
}

// generateIssueIndexMapping generates the bleve index mapping for issues
func generateIssueIndexMapping() (mapping.IndexMapping, error) {
	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.Store = false
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("repo_id", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Store = false
	textFieldMapping.IncludeInAll = false

	boolFieldMapping := bleve.NewBooleanFieldMapping()
	boolFieldMapping.Store = false
	boolFieldMapping.IncludeInAll = false

	numberFieldMapping := bleve.NewNumericFieldMapping()
	numberFieldMapping.Store = false
	numberFieldMapping.IncludeInAll = false

	docMapping.AddFieldMappingsAt("title", textFieldMapping)
	docMapping.AddFieldMappingsAt("content", textFieldMapping)
	docMapping.AddFieldMappingsAt("comments", textFieldMapping)

	// TBC: IndexerData has been changed, but the mapping has not been updated
	docMapping.AddFieldMappingsAt("is_pull", boolFieldMapping)
	docMapping.AddFieldMappingsAt("is_closed", boolFieldMapping)
	docMapping.AddFieldMappingsAt("labels", numberFieldMapping)
	docMapping.AddFieldMappingsAt("no_labels", boolFieldMapping)
	docMapping.AddFieldMappingsAt("milestones", numberFieldMapping)
	docMapping.AddFieldMappingsAt("no_milestones", boolFieldMapping)
	docMapping.AddFieldMappingsAt("projects", numberFieldMapping)
	docMapping.AddFieldMappingsAt("no_projects", boolFieldMapping)
	docMapping.AddFieldMappingsAt("author", numberFieldMapping)
	docMapping.AddFieldMappingsAt("assignee", numberFieldMapping)
	docMapping.AddFieldMappingsAt("mentions", numberFieldMapping)
	docMapping.AddFieldMappingsAt("reviewers", numberFieldMapping)
	docMapping.AddFieldMappingsAt("requested_reviewers", numberFieldMapping)

	docMapping.AddFieldMappingsAt("created_at", numberFieldMapping)
	docMapping.AddFieldMappingsAt("updated_at", numberFieldMapping)
	docMapping.AddFieldMappingsAt("closed_at", numberFieldMapping)
	docMapping.AddFieldMappingsAt("due_date", numberFieldMapping)

	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return nil, err
	} else if err = mapping.AddCustomAnalyzer(issueIndexerAnalyzer, map[string]any{
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
	mapping.DefaultMapping = bleve.NewDocumentDisabledMapping() // disable default mapping, avoid indexing unexpected structs

	return mapping, nil
}

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_bleve.Indexer
	indexer_internal.Indexer // do not composite inner_bleve.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new bleve local indexer
func NewIndexer(indexDir string) *Indexer {
	inner := inner_bleve.NewIndexer(indexDir, issueIndexerLatestVersion, generateIssueIndexMapping)
	return &Indexer{
		Indexer: inner,
		inner:   inner,
	}
}

// Index will save the index data
func (b *Indexer) Index(_ context.Context, issues ...*internal.IndexerData) error {
	batch := inner_bleve.NewFlushingBatch(b.inner.Indexer, maxBatchSize)
	for _, issue := range issues {
		if err := batch.Index(indexer_internal.Base36(issue.ID), (*IndexerData)(issue)); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(_ context.Context, ids ...int64) error {
	batch := inner_bleve.NewFlushingBatch(b.inner.Indexer, maxBatchSize)
	for _, id := range ids {
		if err := batch.Delete(indexer_internal.Base36(id)); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	var repoQueriesP []*query.NumericRangeQuery
	for _, repoID := range options.RepoIDs {
		repoQueriesP = append(repoQueriesP, inner_bleve.NumericEqualityQuery(repoID, "repo_id"))
	}
	repoQueries := make([]query.Query, len(repoQueriesP))
	for i, v := range repoQueriesP {
		repoQueries[i] = query.Query(v)
	}

	indexerQuery := bleve.NewConjunctionQuery(
		bleve.NewDisjunctionQuery(repoQueries...),
		bleve.NewDisjunctionQuery(
			inner_bleve.MatchPhraseQuery(options.Keyword, "title", issueIndexerAnalyzer),
			inner_bleve.MatchPhraseQuery(options.Keyword, "content", issueIndexerAnalyzer),
			inner_bleve.MatchPhraseQuery(options.Keyword, "comments", issueIndexerAnalyzer),
		))
	search := bleve.NewSearchRequestOptions(indexerQuery, options.Limit, options.Skip, false)
	search.SortBy([]string{"-_score"})

	result, err := b.inner.Indexer.SearchInContext(ctx, search)
	if err != nil {
		return nil, err
	}

	ret := &internal.SearchResult{
		Total:     int64(result.Total),
		Hits:      make([]internal.Match, 0, len(result.Hits)),
		Imprecise: true,
	}
	for _, hit := range result.Hits {
		id, err := indexer_internal.ParseBase36(hit.ID)
		if err != nil {
			return nil, err
		}
		ret.Hits = append(ret.Hits, internal.Match{
			ID: id,
		})
	}
	return ret, nil
}
