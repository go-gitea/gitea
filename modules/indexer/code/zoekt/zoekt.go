// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build zoekt && unix

package zoekt

import (
	"bytes"
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

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/analyze"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/internal"
	indexer_internal "gitea.dev/modules/indexer/internal"
	inner_zoekt "gitea.dev/modules/indexer/internal/zoekt"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/typesniffer"

	"github.com/go-enry/go-enry/v2"
	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/index"
	"github.com/sourcegraph/zoekt/query"
)

const repoIndexerLatestVersion = 1

// maxDeltaShards is how many shards a repository may accumulate through delta
// builds before the next index run rebuilds it from scratch. Delta builds stack
// a new shard on top of the existing ones and never compact, so without this a
// frequently pushed repository would end up with one shard per push.
const maxDeltaShards = 20

const (
	// searchShardMaxMatchCount bounds the work a single search does per shard.
	// Zoekt's own default is 100000 which would collect far more candidate
	// matches than the result list can ever show.
	searchShardMaxMatchCount = 10000
	// searchNumContextLines is the number of lines zoekt returns around each
	// matching line. Only the best matching line of a file is displayed, with
	// this many lines of context before and after it.
	searchNumContextLines = 1
	// searchMaxDocDisplayCount bounds the number of files a search reports.
	// Results beyond it are dropped before paging, so the reported total is
	// capped at this value as well.
	searchMaxDocDisplayCount = 1000
)

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

func newZoektIndexOptions(indexDir string, repo *repo_model.Repository, targetSHA string) index.Options {
	opts := index.Options{
		IndexDir: indexDir,
		SizeMax:  int(setting.Indexer.MaxIndexerFileSize),
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
	// the defaults (ctags path, size limits, ...) are part of the option hash
	// stored in the shards, so they have to be applied before comparing states
	opts.SetDefaults()
	return opts
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
		stdout, _, err = gitcmd.NewCommand("cat-file", "-s").AddDynamicArguments(update.BlobSha).WithRepo(repo).RunStdString(ctx)
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
	}
	if !typesniffer.DetectContentType(fileContents).IsText() {
		// FIXME: UTF-16 files will probably fail here
		// Even if the file is not recognized as a "text file", we still index its
		// name to make it searchable, while leaving the content empty.
		fileContents = nil
	}

	// the blob's trailing newline belongs to the shared batch reader and has to be
	// consumed here, also for the blobs whose content was dropped above
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
	opts := newZoektIndexOptions(b.indexDir, repo, sha)

	state, _ := opts.IndexState()
	if state == index.IndexStateEqual {
		// already indexed at this revision with the same options
		return nil
	}

	// A delta build stacks a new shard on top of the existing ones, which zoekt
	// only accepts while the index version and the build options still match.
	// Anything else - a missing, corrupt or stale index, or too many stacked
	// shards - needs a full rebuild from the complete tree instead.
	opts.IsDelta = (state == index.IndexStateContent || state == index.IndexStateMeta) &&
		len(opts.FindAllShards()) <= maxDeltaShards
	if !opts.IsDelta && !changes.Genesis {
		log.Debug("zoekt: rebuilding index of repo %d from scratch, index state is %q", repo.ID, state)
		gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
		if err != nil {
			return err
		}
		defer closer.Close()
		if changes, err = internal.GenesisChanges(ctx, repo, gitRepo, sha); err != nil {
			return fmt.Errorf("listing full tree of repo %d: %w", repo.ID, err)
		}
	}

	builder, err := index.NewBuilder(opts)
	if err != nil {
		return fmt.Errorf("error creating builder: %w", err)
	}

	if err := b.index(ctx, builder, repo, changes); err != nil {
		// Finish has to be called on failure too, it is what drains the in-flight
		// build goroutines and cleans up the temporary shards. The index is left
		// partially updated, the caller does not store the new commit SHA so the
		// next run indexes the same changes again.
		if finishErr := builder.Finish(); finishErr != nil {
			log.Error("zoekt: failed to finish builder of repo %d: %v", repo.ID, finishErr)
		}
		return err
	}

	return builder.Finish()
}

func (b *Indexer) index(ctx context.Context, builder *index.Builder, repo *repo_model.Repository, changes *internal.RepoChanges) error {
	if len(changes.Updates) > 0 {
		gitBatch, err := git.NewBatch(ctx, repo)
		if err != nil {
			return err
		}
		defer gitBatch.Close()

		for _, update := range changes.Updates {
			if err := b.addUpdate(ctx, builder, gitBatch, update, repo); err != nil {
				return err
			}
		}
	}

	for _, filename := range changes.RemovedFilenames {
		b.addDelete(builder, filename)
	}

	return nil
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

// newContentSubstring builds a literal content match for keyword. The query is
// built directly instead of being formatted into zoekt's `content:"..."` syntax,
// because that syntax strips backslash escapes and would let a quote in the
// keyword break out of the string literal.
func newContentSubstring(keyword string) query.Q {
	return &query.Substring{Pattern: keyword, Content: true}
}

// generateZoektQuery creates a Zoekt query object based on search options
func (b *Indexer) generateZoektQuery(_ context.Context, opts *internal.SearchOptions) (query.Q, error) {
	keyword := opts.Keyword
	if keyword == "" {
		return nil, errors.New("empty keyword")
	}

	// Build base content query according to search mode
	var contentQuery query.Q
	var err error

	switch opts.SearchMode {
	case indexer.SearchModeRegexp:
		// Regular expression search mode
		contentQuery, err = query.RegexpQuery(keyword, true /* content */, false /* file */)
		if err != nil {
			return nil, fmt.Errorf("parse regexp keyword %q: %w", keyword, err)
		}

	case indexer.SearchModeWords:
		// Multi-word search mode - words are combined with OR
		fields := strings.Fields(keyword)
		if len(fields) == 0 {
			return nil, errors.New("empty keyword")
		}

		words := make([]query.Q, 0, len(fields))
		for _, field := range fields {
			words = append(words, newContentSubstring(field))
		}
		contentQuery = query.NewOr(words...)

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
		contentQuery = newContentSubstring(keyword)
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

	return query.Simplify(finalQuery), nil
}

func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	var searchResults []*internal.SearchResult

	q, err := b.generateZoektQuery(ctx, opts)
	if err != nil {
		return 0, nil, nil, err
	}

	result, err := b.inner.Searcher.Search(ctx, q, &zoekt.SearchOptions{
		NumContextLines:    searchNumContextLines,
		ShardMaxMatchCount: searchShardMaxMatchCount,
		MaxDocDisplayCount: searchMaxDocDisplayCount,
	})
	if err != nil {
		return 0, nil, nil, err
	}

	// The other search modes query the content only, so a filename match means the
	// query was a zoekt expression like `file:foo` and has to be kept as a result.
	if opts.SearchMode != indexer.SearchModeZoekt {
		for i := range result.Files {
			result.Files[i].LineMatches = slices.DeleteFunc(result.Files[i].LineMatches, func(line zoekt.LineMatch) bool {
				return line.FileName
			})
		}
		result.Files = slices.DeleteFunc(result.Files, func(file zoekt.FileMatch) bool {
			return len(file.LineMatches) == 0
		})
	}

	// Get language statistics from all results before filtering
	searchResultsLanguages := getSearchResultLanguages(result)

	// Apply language filter if specified.
	// "Unknown" is Gitea's representation of files zoekt could not classify (Language=""),
	// so translate back to "" when filtering inside zoekt.
	if opts.Language != "" {
		zoektLang := opts.Language
		if zoektLang == analyze.UnknownLanguage {
			zoektLang = ""
		}
		result.Files = slices.DeleteFunc(result.Files, func(file zoekt.FileMatch) bool {
			return file.Language != zoektLang
		})
	}

	// Record the total number of matched files after all filters have been
	// applied but before pagination, so the caller receives an accurate count.
	total := int64(len(result.Files))

	// Sort results deterministically before pagination.
	// Zoekt uses an unstable sort internally and searches shards concurrently,
	// so equal-scored files arrive in non-deterministic order. Apply a stable
	// sort here using (Score desc, RepoID asc, FileName asc) as the key to
	// guarantee consistent ordering across repeated requests.
	slices.SortStableFunc(result.Files, func(a, b zoekt.FileMatch) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		if c := cmp.Compare(a.RepositoryID, b.RepositoryID); c != 0 {
			return c
		}
		return cmp.Compare(a.FileName, b.FileName)
	})

	// pagination
	if opts.Paginator != nil {
		skip, take := opts.GetSkipTake()

		pageStart := min(skip, len(result.Files))
		pageEnd := min(pageStart+take, len(result.Files))
		result.Files = result.Files[pageStart:pageEnd]
	}

	// Build a snippet per file instead of returning its whole content: only the
	// best matching line is shown, together with searchNumContextLines lines
	// around it, like zoekt-webserver does.
	for _, file := range result.Files {
		// Zoekt sorts the line matches by score, but the order of equal scores
		// is unstable, so break ties on the line number for determinism.
		var best *zoekt.LineMatch
		for i := range file.LineMatches {
			line := &file.LineMatches[i]
			if line.FileName {
				continue
			}
			if best == nil || line.Score > best.Score ||
				(line.Score == best.Score && line.LineNumber < best.LineNumber) {
				best = line
			}
		}

		// a filename-only match has no line or fragment to show
		var content string
		startIndex, endIndex, contentStartLineNum := 0, 0, 1
		if best != nil {
			content = string(best.Before) + string(best.Line) + string(best.After)
			contentStartLineNum = best.LineNumber - bytes.Count(best.Before, []byte{'\n'})
			startIndex, endIndex = -1, -1
			for _, frag := range best.LineFragments {
				fragStart := len(best.Before) + frag.LineOffset
				fragEnd := fragStart + frag.MatchLength
				if startIndex < 0 || fragStart < startIndex {
					startIndex = fragStart
				}
				if endIndex < 0 || fragEnd > endIndex {
					endIndex = fragEnd
				}
			}
			if startIndex < 0 || endIndex < 0 {
				startIndex, endIndex = 0, 0
			}
			// a match may extend past the displayed lines, keep the highlight inside them
			endIndex = min(endIndex, len(content))
		}

		lang := file.Language
		if lang == "" {
			lang = analyze.UnknownLanguage
		}
		searchResults = append(searchResults, &internal.SearchResult{
			Filename:            file.FileName,
			Content:             content,
			ContentStartLineNum: contentStartLineNum,
			RepoID:              int64(file.RepositoryID),
			StartIndex:          startIndex,
			EndIndex:            endIndex,
			Language:            lang,
			Color:               enry.GetColor(lang),
			CommitID:            file.Version,
			// UpdatedUnix: not supported yet,
		})
	}

	return total, searchResults, searchResultsLanguages, nil
}

func getSearchResultLanguages(searchResult *zoekt.SearchResult) []*internal.SearchResultLanguages {
	languages := make(map[string]int)

	for _, file := range searchResult.Files {
		lang := file.Language
		if lang == "" {
			lang = analyze.UnknownLanguage
		}
		languages[lang]++
	}

	searchResultLanguages := make([]*internal.SearchResultLanguages, 0, len(languages))

	for lang, count := range languages {
		searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
			Language: lang,
			Count:    count,
			Color:    enry.GetColor(lang),
		})
	}

	// Sort by file count descending, with language name as a tiebreaker to
	// ensure deterministic ordering when multiple languages have the same count.
	slices.SortFunc(searchResultLanguages, func(a, b *internal.SearchResultLanguages) int {
		if c := cmp.Compare(b.Count, a.Count); c != 0 {
			return c
		}
		return cmp.Compare(a.Language, b.Language)
	})

	return searchResultLanguages
}
