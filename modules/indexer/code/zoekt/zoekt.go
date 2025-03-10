// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package zoekt

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_zoekt "code.gitea.io/gitea/modules/indexer/internal/zoekt"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"

	"github.com/go-enry/go-enry/v2"
	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/index"
	"github.com/sourcegraph/zoekt/query"
)

const repoIndexerLatestVersion = 1

// errIncrementalSkipIndexing is a sentinel error indicating that incremental indexing should be skipped
var errIncrementalSkipIndexing = errors.New("incremental indexing skipped")

type Indexer struct {
	indexer_internal.Indexer // do not composite inner_zoekt.Indexer directly to avoid exposing too much
	inner                    *inner_zoekt.Indexer
	indexDir                 string
}

func (b *Indexer) SupportedSearchModes() []indexer.SearchMode {
	return indexer.ZoektSearchModes()
}

func NewIndexer(indexDir string) *Indexer {
	idxer := inner_zoekt.NewIndexer(indexDir, repoIndexerLatestVersion)
	return &Indexer{
		Indexer:  idxer,
		inner:    idxer,
		indexDir: indexDir,
	}
}

func newZoektIndexBuilder(indexDir string, repo *repo_model.Repository, targetSHA string) (*index.Builder, error) {
	opts := index.Options{
		IndexDir: indexDir,
		SizeMax:  int(setting.Indexer.MaxIndexerFileSize),
		IsDelta:  true,
		RepositoryDescription: zoekt.Repository{
			ID:   uint32(repo.ID),
			Name: strconv.FormatInt(repo.ID, 10),
			Branches: []zoekt.RepositoryBranch{
				{
					Name:    "HEAD",
					Version: targetSHA,
				},
			},
		},
	}

	if opts.IncrementalSkipIndexing() {
		return nil, errIncrementalSkipIndexing
	}

	opts.SetDefaults()

	builder, err := index.NewBuilder(opts)
	if err != nil {
		return nil, fmt.Errorf("index.newZoektIndexBuilder: %w", err)
	}

	return builder, nil
}

func (b *Indexer) addDelete(builder *index.Builder, filename string) {
	builder.MarkFileAsChangedOrRemoved(filename)
}

func (b *Indexer) addUpdate(ctx context.Context, builder *index.Builder, catFileBatch git.CatFileBatch, update internal.FileUpdate, repo *repo_model.Repository) error {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil
	}

	size := update.Size
	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = gitrepo.RunCmdString(ctx, repo, gitcmd.NewCommand("cat-file", "-s").AddDynamicArguments(update.BlobSha))
		if err != nil {
			return err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}
	if size > setting.Indexer.MaxIndexerFileSize {
		b.addDelete(builder, update.Filename)
		return nil
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
		return nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return err
	}

	builder.MarkFileAsChangedOrRemoved(update.Filename)

	// branches := []string{repo.DefaultBranch}
	branches := []string{"HEAD"}

	err = builder.Add(
		index.Document{
			Name:     update.Filename,
			Content:  charset.ToUTF8DropErrors(fileContents),
			Branches: branches,
		})
	if err != nil {
		return fmt.Errorf("error adding document with name %s: %w", update.Filename, err)
	}

	return nil
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	builder, err := newZoektIndexBuilder(b.indexDir, repo, sha)
	if err != nil {
		if errors.Is(err, errIncrementalSkipIndexing) {
			// skip indexing when there is no change
			return nil
		}
		return fmt.Errorf("error creating builder: %w", err)
	}

	if len(changes.Updates) > 0 {
		gitBatch, err := gitrepo.NewBatch(ctx, repo)
		if err != nil {
			return err
		}
		defer gitBatch.Close()

		for _, update := range changes.Updates {
			err := b.addUpdate(ctx, builder, gitBatch, update, repo)
			if err != nil {
				return err
			}
		}
	}

	for _, filename := range changes.RemovedFilenames {
		b.addDelete(builder, filename)
	}

	return builder.Finish()
}

// Delete entries by repoId
func (b *Indexer) Delete(ctx context.Context, repoID int64) error {
	repoPathPrefix := strconv.FormatInt(repoID, 10)

	// remove all {repoId}_v{N}.{X}.zoekt or {repoId}_v{N}.{X}.zoekt.meta where X is %05d formatted int in b.indexDir
	pattern := repoPathPrefix + "_v*.[0-9][0-9][0-9][0-9][0-9].zoekt*"
	matches, err := filepath.Glob(filepath.Join(b.indexDir, pattern))
	if err != nil {
		return fmt.Errorf("finding files to delete: %w", err)
	}

	for _, filePath := range matches {
		if err := os.Remove(filePath); err != nil {
			log.Error("failed to delete %s: %v", filePath, err)
		}
	}

	tmpPattern := repoPathPrefix + "_v*.tmp"
	tmpMatches, err := filepath.Glob(filepath.Join(b.indexDir, tmpPattern))
	if err != nil {
		return fmt.Errorf("finding temp files to delete: %w", err)
	}

	for _, filePath := range tmpMatches {
		if err := os.Remove(filePath); err != nil {
			log.Error("failed to delete temp file %s: %v", filePath, err)
		}
	}

	return nil
}

func TransToZoektContentQueryString(s string) string {
	return fmt.Sprintf("content:\"%s\"", s)
}

// generateZoektQuery creates a Zoekt query object based on search options
func (b *Indexer) generateZoektQuery(_ context.Context, opts *internal.SearchOptions) (query.Q, error) {
	keyword := opts.Keyword

	// Build base content query according to search mode
	var contentQuery query.Q
	var err error

	switch opts.SearchMode {
	case indexer.SearchModeRegexp:
		// Regular expression search mode
		contentQuery, err = query.Parse(TransToZoektContentQueryString(keyword))
		if err != nil {
			return nil, fmt.Errorf("parse regexp keyword %q: %w", keyword, err)
		}

	case indexer.SearchModeWords:
		// Multi-word search mode - words are combined with OR
		fields := strings.Fields(keyword)
		if len(fields) == 0 {
			return nil, errors.New("empty keyword")
		}

		// Process first word
		contentQuery, err = query.Parse(TransToZoektContentQueryString(QuoteMeta(fields[0])))
		if err != nil {
			return nil, fmt.Errorf("parse word keyword %q: %w", fields[0], err)
		}

		// Process remaining words, connecting with OR
		for _, field := range fields[1:] {
			fieldQuery, err := query.Parse(TransToZoektContentQueryString(QuoteMeta(field)))
			if err != nil {
				return nil, fmt.Errorf("parse word keyword %q: %w", field, err)
			}
			contentQuery = query.NewOr(contentQuery, fieldQuery)
		}

	case indexer.SearchModeZoekt:
		// Zoekt search mode - use zoekt syntax
		contentQuery, err = query.Parse(keyword)
		if err != nil {
			return nil, fmt.Errorf("parse zoekt keyword %q: %w", keyword, err)
		}
	case indexer.SearchModeExact:
		fallthrough
	default:
		// Exact match mode (default)
		contentQuery, err = query.Parse(TransToZoektContentQueryString(QuoteMeta(keyword)))
		if err != nil {
			return nil, fmt.Errorf("parse exact keyword %q: %w", keyword, err)
		}
	}

	// Build final query by combining with all filters
	finalQuery := contentQuery

	// Add repository ID filter
	if len(opts.RepoIDs) > 0 {
		repoIDs := make([]uint32, 0, len(opts.RepoIDs))
		for _, repoID := range opts.RepoIDs {
			repoIDs = append(repoIDs, uint32(repoID))
		}
		finalQuery = query.NewAnd(finalQuery, query.NewRepoIDs(repoIDs...))
	}

	log.Info("Search query: %s", finalQuery.String())

	return finalQuery, nil
}

func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	var searchResults []*internal.SearchResult

	q, err := b.generateZoektQuery(ctx, opts)
	if err != nil {
		return 0, nil, nil, err
	}

	result, err := b.inner.Searcher.Search(ctx, q, &zoekt.SearchOptions{
		Whole: true,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	log.Info("len of (result): %d", len(result.Files))

	// remove filename match items from the result
	for i := 0; i < len(result.Files); i++ {
		result.Files[i].LineMatches = slices.DeleteFunc(result.Files[i].LineMatches, func(line zoekt.LineMatch) bool {
			return line.FileName
		})
	}
	result.Files = slices.DeleteFunc(result.Files, func(file zoekt.FileMatch) bool {
		return len(file.LineMatches) == 0
	})

	// Get language statistics from all results before filtering
	searchResultsLanguages := getSearchResultLanguages(result)

	// Apply language filter if specified
	if opts.Language != "" {
		result.Files = slices.DeleteFunc(result.Files, func(file zoekt.FileMatch) bool {
			return file.Language != opts.Language
		})
	}

	// pagination
	if opts.Paginator != nil {
		page, pageSize := opts.GetSkipTake()

		pageStart := min(page*pageSize, len(result.Files))
		pageEnd := min((page+1)*pageSize, len(result.Files))
		result.Files = result.Files[pageStart:pageEnd]
	}

	// calculate highlight range
	for _, file := range result.Files {
		startIndex, endIndex := -1, -1
		for _, line := range file.LineMatches {
			for _, frag := range line.LineFragments {
				fragStart := (int)(frag.Offset)
				fragEnd := (int)(frag.Offset) + frag.MatchLength
				if startIndex < 0 || fragStart < startIndex {
					startIndex = fragStart
				}
				if endIndex < 0 || fragEnd > endIndex {
					endIndex = fragEnd
				}
			}
		}

		searchResults = append(searchResults, &internal.SearchResult{
			Filename:   file.FileName,
			Content:    string(file.Content),
			RepoID:     int64(file.RepositoryID),
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Language:   file.Language,
			Color:      enry.GetColor(file.Language),
			CommitID:   file.Version,
			// UpdatedUnix: not supported yet,
		})
	}

	return int64(result.Stats.FileCount), searchResults, searchResultsLanguages, nil
}

func getSearchResultLanguages(searchResult *zoekt.SearchResult) []*internal.SearchResultLanguages {
	languages := make(map[string]int)

	for _, file := range searchResult.Files {
		languages[file.Language]++
	}

	searchResultLanguages := make([]*internal.SearchResultLanguages, 0, len(languages))

	for lang, count := range languages {
		searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
			Language: lang,
			Count:    count,
			Color:    enry.GetColor(lang),
		})
	}

	// Sort by file count in descending order
	slices.SortFunc(searchResultLanguages, func(a, b *internal.SearchResultLanguages) int {
		return cmp.Compare(b.Count, a.Count)
	})

	return searchResultLanguages
}
