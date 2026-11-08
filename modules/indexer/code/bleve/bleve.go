// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/analyze"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/internal"
	indexer_internal "gitea.dev/modules/indexer/internal"
	inner_bleve "gitea.dev/modules/indexer/internal/bleve"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/util"

	"github.com/blevesearch/bleve/v2"
	analyzer_custom "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	analyzer_keyword "github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/registry"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/go-enry/go-enry/v2"
)

const (
	unicodeNormalizeName = "unicodeNormalize"
	maxBatchSize         = 16
)

func addUnicodeNormalizeTokenFilter(m *mapping.IndexMappingImpl) error {
	return m.AddCustomTokenFilter(unicodeNormalizeName, map[string]any{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	})
}

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
	filenameIndexerAnalyzer  = "filenameIndexerAnalyzer"
	repoIndexerDocType       = "repoIndexerDocType"
	repoIndexerLatestVersion = 12
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

	fileNamedMapping := bleve.NewTextFieldMapping()
	fileNamedMapping.IncludeInAll = false
	fileNamedMapping.Analyzer = filenameIndexerAnalyzer
	docMapping.AddFieldMappingsAt("Filename", fileNamedMapping)

	termFieldMapping := bleve.NewTextFieldMapping()
	termFieldMapping.IncludeInAll = false
	termFieldMapping.Analyzer = analyzer_keyword.Name
	docMapping.AddFieldMappingsAt("Language", termFieldMapping)
	docMapping.AddFieldMappingsAt("CommitID", termFieldMapping)

	timeFieldMapping := bleve.NewDateTimeFieldMapping()
	timeFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("UpdatedAt", timeFieldMapping)

	mapping := bleve.NewIndexMapping()

	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return nil, err
	} else if err := mapping.AddCustomAnalyzer(repoIndexerAnalyzer, map[string]any{
		"type":          analyzer_custom.Name,
		"char_filters":  []string{},
		"tokenizer":     codeTokenizerName,
		"token_filters": []string{},
	}); err != nil {
		return nil, err
	}

	if err := mapping.AddCustomAnalyzer(filenameIndexerAnalyzer, map[string]any{
		"type":          analyzer_custom.Name,
		"char_filters":  []string{},
		"tokenizer":     pathTokenizerName,
		"token_filters": []string{unicodeNormalizeName, lowercase.Name},
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

func (b *Indexer) SupportedSearchModes() []indexer.SearchMode {
	return indexer.SearchModesExactWords()
}

func init() {
	// due to bleve's design problem, the "Register" must be done in the main goroutine, otherwise data-race
	util.MustNoError(registry.RegisterTokenizer(codeTokenizerName, codeTokenizerConstructor))
	util.MustNoError(registry.RegisterTokenizer(pathTokenizerName, pathTokenizerConstructor))
}

// NewIndexer creates a new bleve local indexer
func NewIndexer(indexDir string) *Indexer {
	inner := inner_bleve.NewIndexer(indexDir, repoIndexerLatestVersion, generateBleveIndexMapping)
	return &Indexer{
		Indexer: inner,
		inner:   inner,
	}
}

func (b *Indexer) addUpdate(ctx context.Context, catFileBatch git.CatFileBatch, commitSha string,
	update internal.FileUpdate, repo *repo_model.Repository, batch *inner_bleve.FlushingBatch,
) error {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil
	}

	size := update.Size

	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = gitcmd.NewCommand("cat-file", "-s").AddDynamicArguments(update.BlobSha).WithRepo(repo).RunStdString(ctx)
		if err != nil {
			return err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}

	if size > setting.Indexer.MaxIndexerFileSize {
		return b.addDelete(update.Filename, repo, batch)
	}

	info, batchReader, err := catFileBatch.QueryContent(update.BlobSha)
	if err != nil {
		return err
	}
	fileContents, err := io.ReadAll(io.LimitReader(batchReader, info.Size))
	if err != nil {
		return err
	} else if !typesniffer.DetectContentType(fileContents).IsText() {
		// FIXME: UTF-16 files will probably fail here
		// Even if the file is not recognized as a "text file", we could still put its name into the indexers to make the filename become searchable, while leave the content to empty.
		fileContents = nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return err
	}
	id := internal.FilenameIndexerID(repo.ID, update.Filename)
	return batch.Index(id, &RepoIndexerData{
		RepoID:    repo.ID,
		CommitID:  commitSha,
		Filename:  update.Filename,
		Content:   string(charset.ToUTF8DropErrors(fileContents)),
		Language:  analyze.GetCodeLanguage(update.Filename, fileContents),
		UpdatedAt: time.Now().UTC(),
	})
}

func (b *Indexer) addDelete(filename string, repo *repo_model.Repository, batch *inner_bleve.FlushingBatch) error {
	id := internal.FilenameIndexerID(repo.ID, filename)
	return batch.Delete(id)
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
			if err := b.addUpdate(ctx, catfileBatch, sha, update, repo, batch); err != nil {
				return err
			}
		}
	}
	for _, filename := range changes.RemovedFilenames {
		if err := b.addDelete(filename, repo, batch); err != nil {
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

// Search searches for files in the specified repo.
// Returns the matching file-paths
func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	var (
		indexerQuery query.Query
		keywordQuery query.Query
		contentQuery query.Query
	)

	pathQuery := bleve.NewPrefixQuery(strings.ToLower(opts.Keyword))
	pathQuery.FieldVal = "Filename"
	pathQuery.SetBoost(10)

	terms := internal.SearchTerms(opts)
	if len(terms) == 0 {
		return 0, nil, nil, nil
	}
	contentQueries := make([]query.Query, 0, len(terms))
	for _, term := range terms {
		contentQueries = append(contentQueries, inner_bleve.MatchAndQuery(term, "Content", repoIndexerAnalyzer, 0))
	}
	contentQuery = bleve.NewConjunctionQuery(contentQueries...)

	keywordQuery = bleve.NewDisjunctionQuery(contentQuery, pathQuery)

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
	if !slices.ContainsFunc(terms, func(term string) bool { return utf8.RuneCountInString(term) > 3 }) {
		matched, scanned = from, from
	}
	var result *bleve.SearchResult
	var total int64
	searchResults := make([]*internal.SearchResult, 0, pageSize)
	for {
		size := from + pageSize - matched
		searchRequest := bleve.NewSearchRequestOptions(indexerQuery, size, scanned, false)
		searchRequest.Fields = []string{"Content", "Filename", "RepoID", "Language", "CommitID", "UpdatedAt"}
		searchRequest.IncludeLocations = true
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
			repoID, okRepoID := hit.Fields["RepoID"].(float64)
			if !okContent || !okLanguage || !okCommitID || !okUpdatedAt || !okRepoID {
				hitFieldsJson, _ := json.Marshal(hit.Fields)
				setting.PanicInDevOrTesting("unexpected field types in search hit %q: %s", hit.ID, string(hitFieldsJson))
				return 0, nil, nil, fmt.Errorf("unexpected field types in search hit %q", hit.ID)
			}

			var startIndex, endIndex int
			if len(hit.Locations["Filename"]) > 0 {
				startIndex, endIndex = internal.FilenameMatchIndexPos(content)
			} else if startIndex, endIndex = internal.ContentMatchIndexPos(content, terms); startIndex < 0 {
				continue
			}
			matched++
			if matched <= from {
				continue
			}

			var updatedUnix timeutil.TimeStamp
			if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
				updatedUnix = timeutil.TimeStamp(t.Unix())
			}
			searchResults = append(searchResults, &internal.SearchResult{
				RepoID:      int64(repoID),
				StartIndex:  startIndex,
				EndIndex:    endIndex,
				Filename:    internal.FilenameOfIndexerID(hit.ID),
				Content:     content,
				CommitID:    commitID,
				UpdatedUnix: updatedUnix,
				Language:    language,
				Color:       enry.GetColor(language),
			})
		}
		if matched == from+pageSize || len(batch.Hits) < size {
			total = max(int64(matched), int64(matched)+int64(batch.Total)-int64(scanned))
			break
		}
	}
	if total == 0 {
		return 0, nil, nil, nil
	}

	searchResultLanguages := make([]*internal.SearchResultLanguages, 0, 10)
	if len(opts.Language) > 0 {
		// Use separate query to go get all language counts
		facetRequest := bleve.NewSearchRequestOptions(facetQuery, 1, 0, false)
		facetRequest.Fields = []string{"Content", "RepoID", "Language", "CommitID", "UpdatedAt"}
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
