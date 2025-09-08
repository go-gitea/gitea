// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer"
	path_filter "code.gitea.io/gitea/modules/indexer/code/bleve/token/path"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_bleve "code.gitea.io/gitea/modules/indexer/internal/bleve"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"

	"github.com/blevesearch/bleve/v2"
	analyzer_custom "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	analyzer_keyword "github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/letter"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/mapping"
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
	filenameIndexerTokenizer = "filenameIndexerTokenizer"
	repoIndexerDocType       = "repoIndexerDocType"
	repoIndexerLatestVersion = 9
)

// generateBleveIndexMapping generates a bleve index mapping for the repo indexer
func generateBleveIndexMapping() (mapping.IndexMapping, error) {
	docMapping := bleve.NewDocumentMapping()
	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.IncludeInAll = false
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
		"tokenizer":     letter.Name,
		"token_filters": []string{unicodeNormalizeName, lowercase.Name},
	}); err != nil {
		return nil, err
	}

	if err := mapping.AddCustomAnalyzer(filenameIndexerAnalyzer, map[string]any{
		"type":          analyzer_custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, path_filter.Name, lowercase.Name},
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

// NewIndexer creates a new bleve local indexer
func NewIndexer(indexDir string) *Indexer {
	inner := inner_bleve.NewIndexer(indexDir, repoIndexerLatestVersion, generateBleveIndexMapping)
	return &Indexer{
		Indexer: inner,
		inner:   inner,
	}
}

func (b *Indexer) addUpdate(ctx context.Context, batchWriter git.WriteCloserError, batchReader *bufio.Reader, commitSha string,
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
		stdout, _, err = git.NewCommand("cat-file", "-s").AddDynamicArguments(update.BlobSha).RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})
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

	if _, err := batchWriter.Write([]byte(update.BlobSha + "\n")); err != nil {
		return err
	}

	_, _, size, err = git.ReadBatchLine(batchReader)
	if err != nil {
		return err
	}

	fileContents, err := io.ReadAll(io.LimitReader(batchReader, size))
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
		Content:   string(charset.ToUTF8DropErrors(fileContents, charset.ConvertOpts{})),
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
		gitBatch, err := git.NewBatch(ctx, repo.RepoPath())
		if err != nil {
			return err
		}
		defer gitBatch.Close()

		for _, update := range changes.Updates {
			if err := b.addUpdate(ctx, gitBatch.Writer, gitBatch.Reader, sha, update, repo, batch); err != nil {
				return err
			}
		}
		gitBatch.Close()
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

	searchMode := util.IfZero(opts.SearchMode, b.SupportedSearchModes()[0].ModeValue)
	if searchMode == indexer.SearchModeExact {
		// 1.21 used NewPrefixQuery, but it seems not working well, and later releases changed to NewMatchPhraseQuery
		q := bleve.NewMatchPhraseQuery(opts.Keyword)
		q.Analyzer = repoIndexerAnalyzer
		q.FieldVal = "Content"
		contentQuery = q
	} else /* words */ {
		q := bleve.NewMatchQuery(opts.Keyword)
		q.FieldVal = "Content"
		q.Analyzer = repoIndexerAnalyzer
		if searchMode == indexer.SearchModeFuzzy {
			// this logic doesn't seem right, it is only used to pass the test-case `Keyword:    "dESCRIPTION"`, which doesn't seem to be a real-life use-case.
			q.Fuzziness = inner_bleve.GuessFuzzinessByKeyword(opts.Keyword)
		} else {
			q.Operator = query.MatchQueryOperatorAnd
		}
		contentQuery = q
	}

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
	searchRequest := bleve.NewSearchRequestOptions(indexerQuery, pageSize, from, false)
	searchRequest.Fields = []string{"Content", "Filename", "RepoID", "Language", "CommitID", "UpdatedAt"}
	searchRequest.IncludeLocations = true

	if len(opts.Language) == 0 {
		searchRequest.AddFacet("languages", bleve.NewFacetRequest("Language", 10))
	}

	searchRequest.SortBy([]string{"-_score", "UpdatedAt"})

	result, err := b.inner.Indexer.SearchInContext(ctx, searchRequest)
	if err != nil {
		return 0, nil, nil, err
	}

	total := int64(result.Total)

	searchResults := make([]*internal.SearchResult, len(result.Hits))
	for i, hit := range result.Hits {
		startIndex, endIndex := -1, -1
		for _, locations := range hit.Locations["Content"] {
			location := locations[0]
			locationStart := int(location.Start)
			locationEnd := int(location.End)
			if startIndex < 0 || locationStart < startIndex {
				startIndex = locationStart
			}
			if endIndex < 0 || locationEnd > endIndex {
				endIndex = locationEnd
			}
		}
		if len(hit.Locations["Filename"]) > 0 {
			startIndex, endIndex = internal.FilenameMatchIndexPos(hit.Fields["Content"].(string))
		}

		language := hit.Fields["Language"].(string)
		var updatedUnix timeutil.TimeStamp
		if t, err := time.Parse(time.RFC3339, hit.Fields["UpdatedAt"].(string)); err == nil {
			updatedUnix = timeutil.TimeStamp(t.Unix())
		}
		searchResults[i] = &internal.SearchResult{
			RepoID:      int64(hit.Fields["RepoID"].(float64)),
			StartIndex:  startIndex,
			EndIndex:    endIndex,
			Filename:    internal.FilenameOfIndexerID(hit.ID),
			Content:     hit.Fields["Content"].(string),
			CommitID:    hit.Fields["CommitID"].(string),
			UpdatedUnix: updatedUnix,
			Language:    language,
			Color:       enry.GetColor(language),
		}
	}

	searchResultLanguages := make([]*internal.SearchResultLanguages, 0, 10)
	if len(opts.Language) > 0 {
		// Use separate query to go get all language counts
		facetRequest := bleve.NewSearchRequestOptions(facetQuery, 1, 0, false)
		facetRequest.Fields = []string{"Content", "RepoID", "Language", "CommitID", "UpdatedAt"}
		facetRequest.IncludeLocations = true
		facetRequest.AddFacet("languages", bleve.NewFacetRequest("Language", 10))

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
