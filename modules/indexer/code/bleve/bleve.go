// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/analyze"
	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer/code/internal"
	indexer_internal "gitea.dev/modules/indexer/internal"
	inner_bleve "gitea.dev/modules/indexer/internal/bleve"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"github.com/blevesearch/bleve/v2"
	analyzer_custom "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	analyzer_keyword "github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/registry"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/go-enry/go-enry/v2"
	"golang.org/x/text/unicode/norm"
)

const maxBatchSize = 16

// RepoIndexerData data stored in the repo indexer
type RepoIndexerData struct {
	RepoID    int64
	CommitID  string
	Content   string
	Filename  string
	Language  string
	UpdatedAt time.Time
}

// Type returns the document type, for bleve's mapping.Classifier interface.
func (d *RepoIndexerData) Type() string {
	return repoIndexerDocType
}

const (
	repoIndexerAnalyzer      = "repoIndexerAnalyzer"
	repoIndexerDocType       = "repoIndexerDocType"
	repoIndexerLatestVersion = 13
)

// generateBleveIndexMapping generates a bleve index mapping for the repo indexer
func generateBleveIndexMapping() (mapping.IndexMapping, error) {
	docMapping := bleve.NewDocumentMapping()
	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.IncludeInAll = false
	textFieldMapping.DocValues = false
	textFieldMapping.IncludeTermVectors = false
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)

	docMapping.AddFieldMappingsAt("Filename", textFieldMapping)
	filenamePathMapping := bleve.NewKeywordFieldMapping()
	filenamePathMapping.Name = "FilenamePath"
	filenamePathMapping.IncludeInAll = false
	filenamePathMapping.Store = false
	filenamePathMapping.DocValues = false
	filenamePathMapping.IncludeTermVectors = false
	docMapping.AddFieldMappingsAt("Filename", filenamePathMapping)

	termFieldMapping := bleve.NewTextFieldMapping()
	termFieldMapping.IncludeInAll = false
	termFieldMapping.Analyzer = analyzer_keyword.Name
	docMapping.AddFieldMappingsAt("Language", termFieldMapping)
	docMapping.AddFieldMappingsAt("CommitID", termFieldMapping)

	timeFieldMapping := bleve.NewDateTimeFieldMapping()
	timeFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("UpdatedAt", timeFieldMapping)

	mapping := bleve.NewIndexMapping()

	if err := mapping.AddCustomAnalyzer(repoIndexerAnalyzer, map[string]any{
		"type":          analyzer_custom.Name,
		"char_filters":  []string{},
		"tokenizer":     codeTokenizerName,
		"token_filters": []string{},
	}); err != nil {
		return nil, err
	}

	mapping.DefaultAnalyzer = repoIndexerAnalyzer
	mapping.AddDocumentMapping(repoIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())

	return mapping, nil
}

var _ internal.Indexer = &Indexer{}

// Indexer represents a bleve indexer implementation
type Indexer struct {
	inner                    *inner_bleve.Indexer
	indexer_internal.Indexer // do not composite inner_bleve.Indexer directly to avoid exposing too much
}

func init() {
	// due to bleve's design problem, the "Register" must be done in the main goroutine, otherwise data-race
	util.MustNoError(registry.RegisterTokenizer(codeTokenizerName, codeTokenizerConstructor))
}

// NewIndexer creates a new bleve local indexer
func NewIndexer(indexDir string) *Indexer {
	inner := inner_bleve.NewIndexer(indexDir, repoIndexerLatestVersion, generateBleveIndexMapping)
	return &Indexer{
		Indexer: inner,
		inner:   inner,
	}
}

func (b *Indexer) addUpdate(catFileBatch git.CatFileBatch, commitSha string,
	update internal.FileUpdate, repo *repo_model.Repository, batch *inner_bleve.FlushingBatch,
) error {
	fileContents, searchable, err := internal.ReadFile(catFileBatch, update)
	if err != nil {
		return err
	}
	if !searchable {
		return batch.Delete(internal.FilenameIndexerID(repo.ID, update.Filename))
	}
	id := internal.FilenameIndexerID(repo.ID, update.Filename)
	return batch.Index(id, &RepoIndexerData{
		RepoID:    repo.ID,
		CommitID:  commitSha,
		Filename:  norm.NFC.String(update.Filename),
		Content:   string(fileContents),
		Language:  analyze.GetCodeLanguage(update.Filename, fileContents),
		UpdatedAt: time.Now().UTC(),
	})
}

// Index indexes the data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	batch := inner_bleve.NewFlushingBatch(b.inner.Indexer, maxBatchSize)
	if len(changes.Updates) > 0 {
		catfileBatch, err := git.NewBatch(ctx, repo)
		if err != nil {
			return err
		}
		defer catfileBatch.Close()

		for _, update := range changes.Updates {
			if err := b.addUpdate(catfileBatch, sha, update, repo, batch); err != nil {
				return err
			}
		}
	}
	for _, filename := range changes.RemovedFilenames {
		if err := batch.Delete(internal.FilenameIndexerID(repo.ID, filename)); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(_ context.Context, repoID int64) error {
	query := inner_bleve.NumericEqualityQuery(repoID, "RepoID")
	searchRequest := bleve.NewSearchRequestOptions(query, 2147483647, 0, false)
	result, err := b.inner.Indexer.Search(searchRequest)
	if err != nil {
		return err
	}
	batch := inner_bleve.NewFlushingBatch(b.inner.Indexer, maxBatchSize)
	for _, hit := range result.Hits {
		if err = batch.Delete(hit.ID); err != nil {
			return err
		}
	}
	return batch.Flush()
}

func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	var indexerQuery query.Query
	terms := opts.Terms
	if len(terms) == 0 {
		return 0, nil, nil, nil
	}
	matcher := internal.LiteralMatcher(terms)
	termQueries := make([]query.Query, 0, len(terms))
	for _, term := range terms {
		termQueries = append(termQueries, bleve.NewDisjunctionQuery(
			inner_bleve.MatchAndQuery(term, "Content", repoIndexerAnalyzer, 0), inner_bleve.MatchAndQuery(norm.NFC.String(term), "Filename", repoIndexerAnalyzer, 0),
		))
	}
	keywordQuery := bleve.NewConjunctionQuery(termQueries...)

	if len(opts.RepoIDs) > 0 {
		repoQueries := make([]query.Query, 0, len(opts.RepoIDs))
		for _, repoID := range opts.RepoIDs {
			repoQueries = append(repoQueries, inner_bleve.NumericEqualityQuery(repoID, "RepoID"))
		}

		indexerQuery = bleve.NewConjunctionQuery(
			bleve.NewDisjunctionQuery(repoQueries...),
			keywordQuery,
		)
	} else {
		indexerQuery = keywordQuery
	}

	if opts.Path != "" {
		directoryQuery := bleve.NewPrefixQuery(norm.NFC.String(opts.Path) + "/")
		directoryQuery.SetField("FilenamePath")
		indexerQuery = bleve.NewConjunctionQuery(indexerQuery, directoryQuery)
	}

	// Save for reuse without language filter
	facetQuery := indexerQuery
	if len(opts.Language) > 0 {
		languageQuery := bleve.NewMatchQuery(opts.Language)
		languageQuery.FieldVal = "Language"
		languageQuery.Analyzer = analyzer_keyword.Name

		indexerQuery = bleve.NewConjunctionQuery(
			indexerQuery,
			languageQuery,
		)
	}

	from, pageSize := opts.GetSkipTake()
	// grams prove terms of up to 3 runes, longer ones are verified against the content, so fetch candidates until the page is full
	matched, scanned := 0, 0
	shortTerms := !slices.ContainsFunc(terms, func(term string) bool { return utf8.RuneCountInString(term) > 3 })
	if shortTerms {
		matched, scanned = from, from
	}
	var result *bleve.SearchResult
	var total int64
	verifiedLanguages := make(map[string]int)
	var exhausted bool
	searchResults := make([]*internal.SearchResult, 0, pageSize)
	for {
		size := from + pageSize - matched
		searchRequest := bleve.NewSearchRequestOptions(indexerQuery, size, scanned, false)
		searchRequest.Fields = []string{"Content", "Language", "CommitID", "UpdatedAt"}
		if result == nil && len(opts.Language) == 0 {
			searchRequest.AddFacet("languages", bleve.NewFacetRequest("Language", 10))
		}
		searchRequest.SortBy([]string{"-_score", "UpdatedAt"})

		batch, err := b.inner.Indexer.SearchInContext(ctx, searchRequest)
		if err != nil {
			return 0, nil, nil, err
		}
		if result == nil {
			result = batch
		}

		for _, hit := range batch.Hits {
			scanned++
			content, okContent := hit.Fields["Content"].(string)
			language, okLanguage := hit.Fields["Language"].(string)
			commitID, okCommitID := hit.Fields["CommitID"].(string)
			updatedAt, okUpdatedAt := hit.Fields["UpdatedAt"].(string)
			if !okContent || !okLanguage || !okCommitID || !okUpdatedAt {
				hitFieldsJson, _ := json.Marshal(hit.Fields)
				setting.PanicInDevOrTesting("unexpected field types in search hit %q: %s", hit.ID, string(hitFieldsJson))
				return 0, nil, nil, fmt.Errorf("unexpected field types in search hit %q", hit.ID)
			}

			repoID, filename := internal.ParseIndexerID(hit.ID)
			searchResult := &internal.SearchResult{RepoID: repoID, Filename: filename, Content: content, CommitID: commitID, Language: language}
			if !matcher.MatchResult(searchResult) {
				continue
			}

			matched++
			verifiedLanguages[language]++
			if matched <= from {
				continue
			}

			if updated, err := time.Parse(time.RFC3339, updatedAt); err == nil {
				searchResult.UpdatedUnix = timeutil.TimeStamp(updated.Unix())
			}
			searchResults = append(searchResults, searchResult)
		}
		exhausted = len(batch.Hits) < size || uint64(scanned) >= batch.Total
		if matched == from+pageSize || exhausted {
			if shortTerms {
				total = int64(batch.Total)
			} else {
				total = max(int64(matched), int64(matched)+int64(batch.Total)-int64(scanned))
			}
			break
		}
	}
	if total == 0 {
		return 0, nil, nil, nil
	}

	searchResultLanguages := make([]*internal.SearchResultLanguages, 0, 10)
	if len(opts.Language) == 0 && !shortTerms && exhausted {
		for language, count := range verifiedLanguages {
			if language != "" {
				searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
					Language: language,
					Color:    enry.GetColor(language),
					Count:    count,
				})
			}
		}
		slices.SortFunc(searchResultLanguages, func(first, second *internal.SearchResultLanguages) int {
			if first.Count == second.Count {
				return cmp.Compare(first.Language, second.Language)
			}
			return cmp.Compare(second.Count, first.Count)
		})
		return total, searchResults, searchResultLanguages[:min(10, len(searchResultLanguages))], nil
	}
	if len(opts.Language) > 0 {
		// Use separate query to go get all language counts
		facetRequest := bleve.NewSearchRequestOptions(facetQuery, 1, 0, false)
		facetRequest.Fields = []string{"Content", "Language", "CommitID", "UpdatedAt"}
		facetRequest.AddFacet("languages", bleve.NewFacetRequest("Language", 10))

		var err error
		if result, err = b.inner.Indexer.Search(facetRequest); err != nil {
			return 0, nil, nil, err
		}
	}
	languagesFacet := result.Facets["languages"]
	for _, term := range languagesFacet.Terms.Terms() {
		if len(term.Term) == 0 {
			continue
		}
		searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
			Language: term.Term,
			Color:    enry.GetColor(term.Term),
			Count:    term.Count,
		})
	}
	return total, searchResults, searchResultLanguages, nil
}
