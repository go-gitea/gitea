// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/typesniffer"

	bulkV8 "github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	searchV8 "github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	someV8 "github.com/elastic/go-elasticsearch/v8/typedapi/some"
	typesV8 "github.com/elastic/go-elasticsearch/v8/typedapi/types"
	highlightertypeV8 "github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/highlightertype"
	sortorderV8 "github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	termvectoroptionV8 "github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/termvectoroption"
	textquerytypeV8 "github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/textquerytype"
	"github.com/go-enry/go-enry/v2"
)

const (
	esRepoIndexerLatestVersion = 1
)

var _ internal.Indexer = &IndexerV8{}

// IndexerV8 implements Indexer interface
type IndexerV8 struct {
	inner                    *inner_elasticsearch.IndexerV8
	indexer_internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexerV8(url, indexerName string) *IndexerV8 {
	inner := inner_elasticsearch.NewIndexerV8(url, indexerName, esRepoIndexerLatestVersion, defaultMappingV8)
	indexer := &IndexerV8{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

var defaultMappingV8 = &typesV8.TypeMapping{
	Properties: map[string]typesV8.Property{
		"repo_id": typesV8.NewLongNumberProperty(),
		"content": &typesV8.TextProperty{
			Fields:     make(map[string]typesV8.Property, 0),
			Meta:       make(map[string]string, 0),
			Properties: make(map[string]typesV8.Property, 0),
			TermVector: &termvectoroptionV8.Withpositions,
		},
		"commit_id":  typesV8.NewKeywordProperty(),
		"language":   typesV8.NewKeywordProperty(),
		"updated_at": typesV8.NewLongNumberProperty(),
	},
}

func (b *IndexerV8) addUpdate(ctx context.Context, blk *bulkV8.Bulk, batchWriter git.WriteCloserError, batchReader *bufio.Reader, sha string, update internal.FileUpdate, repo *repo_model.Repository) error {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil
	}

	size := update.Size
	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = git.NewCommand(ctx, "cat-file", "-s").AddDynamicArguments(update.BlobSha).RunStdString(&git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			return err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}

	if size > setting.Indexer.MaxIndexerFileSize {
		return b.addDelete(blk, update.Filename, repo)
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
		return nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return err
	}
	id := internal.FilenameIndexerID(repo.ID, update.Filename)

	return blk.IndexOp(typesV8.IndexOperation{
		Index_: someV8.String(b.inner.VersionedIndexName()),
		Id_:    someV8.String(id),
	}, map[string]any{
		"id":         id,
		"repo_id":    repo.ID,
		"content":    string(fileContents),
		"commit_id":  sha,
		"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
		"updated_at": timeutil.TimeStampNow(),
	})
}

func (b *IndexerV8) addDelete(blk *bulkV8.Bulk, filename string, repo *repo_model.Repository) error {
	id := internal.FilenameIndexerID(repo.ID, filename)
	return blk.DeleteOp(typesV8.DeleteOperation{
		Index_: someV8.String(b.inner.VersionedIndexName()),
		Id_:    someV8.String(id),
	})
}

// Index will save the index data
func (b *IndexerV8) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	if len(changes.Updates) == 0 && len(changes.RemovedFilenames) == 0 {
		return nil
	}

	blk := b.inner.Client.Bulk().Index(b.inner.VersionedIndexName())

	if len(changes.Updates) > 0 {
		// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
		if err := git.EnsureValidGitRepository(ctx, repo.RepoPath()); err != nil {
			log.Error("Unable to open git repo: %s for %-v: %v", repo.RepoPath(), repo, err)
			return err
		}

		batchWriter, batchReader, cancel := git.CatFileBatch(ctx, repo.RepoPath())
		defer cancel()

		for _, update := range changes.Updates {
			err := b.addUpdate(ctx, blk, batchWriter, batchReader, sha, update, repo)
			if err != nil {
				return err
			}
		}
		cancel()
	}

	for _, filename := range changes.RemovedFilenames {
		err := b.addDelete(blk, filename, repo)
		if err != nil {
			return err
		}
	}

	_, err := blk.Do(ctx)
	return err
}

// Delete deletes indexes by ids
func (b *IndexerV8) Delete(ctx context.Context, repoID int64) error {
	_, err := b.inner.Client.DeleteByQuery(b.inner.VersionedIndexName()).
		Query(&typesV8.Query{
			Term: map[string]typesV8.TermQuery{
				"repo_id": {Value: repoID},
			},
		}).
		// Query(elastic.NewTermsQuery("repo_id", repoID)).
		Do(ctx)
	return err
}

// indexPos find words positions for start and the following end on content. It will
// return the beginning position of the first start and the ending position of the
// first end following the start string.
// If not found any of the positions, it will return -1, -1.
func indexPos(content, start, end string) (int, int) {
	startIdx := strings.Index(content, start)
	if startIdx < 0 {
		return -1, -1
	}
	endIdx := strings.Index(content[startIdx+len(start):], end)
	if endIdx < 0 {
		return -1, -1
	}
	return startIdx, startIdx + len(start) + endIdx + len(end)
}

func convertResultV8(searchResult *searchV8.Response, kw string, pageSize int) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	hits := make([]*internal.SearchResult, 0, pageSize)
	for _, hit := range searchResult.Hits.Hits {
		// FIXME: There is no way to get the position the keyword on the content currently on the same request.
		// So we get it from content, this may made the query slower. See
		// https://discuss.elastic.co/t/fetching-position-of-keyword-in-matched-document/94291
		var startIndex, endIndex int
		c, ok := hit.Highlight["content"]
		if ok && len(c) > 0 {
			// FIXME: Since the highlighting content will include <em> and </em> for the keywords,
			// now we should find the positions. But how to avoid html content which contains the
			// <em> and </em> tags? If elastic search has handled that?
			startIndex, endIndex = indexPos(c[0], "<em>", "</em>")
			if startIndex == -1 {
				panic(fmt.Sprintf("1===%s,,,%#v,,,%s", kw, hit.Highlight, c[0]))
			}
		} else {
			panic(fmt.Sprintf("2===%#v", hit.Highlight))
		}

		repoID, fileName := internal.ParseIndexerID(hit.Id_)
		res := make(map[string]any)
		if err := json.Unmarshal(hit.Source_, &res); err != nil {
			return 0, nil, nil, err
		}

		language := res["language"].(string)

		hits = append(hits, &internal.SearchResult{
			RepoID:      repoID,
			Filename:    fileName,
			CommitID:    res["commit_id"].(string),
			Content:     res["content"].(string),
			UpdatedUnix: timeutil.TimeStamp(res["updated_at"].(float64)),
			Language:    language,
			StartIndex:  startIndex,
			EndIndex:    endIndex - 9, // remove the length <em></em> since we give Content the original data
			Color:       enry.GetColor(language),
		})
	}

	return searchResult.Hits.Total.Value, hits, extractAggregates(searchResult), nil
}

func extractAggregates(searchResult *searchV8.Response) []*internal.SearchResultLanguages {
	var searchResultLanguages []*internal.SearchResultLanguages
	agg, found := searchResult.Aggregations["language"]
	if found {
		searchResultLanguages = make([]*internal.SearchResultLanguages, 0, 10)

		languageAgg := agg.(*typesV8.StringTermsAggregate)
		buckets := languageAgg.Buckets.([]typesV8.StringTermsBucket)
		for _, bucket := range buckets {
			searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
				Language: bucket.Key.(string),
				Color:    enry.GetColor(bucket.Key.(string)),
				Count:    int(bucket.DocCount),
			})
		}
	}
	return searchResultLanguages
}

// Search searches for codes and language stats by given conditions.
func (b *IndexerV8) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	// searchType := esMultiMatchTypePhrasePrefix
	searchType := &textquerytypeV8.Phraseprefix
	if opts.IsKeywordFuzzy {
		searchType = &textquerytypeV8.Bestfields
	}

	kwQuery := typesV8.Query{
		MultiMatch: &typesV8.MultiMatchQuery{
			Query:  opts.Keyword,
			Fields: []string{"content"},
			Type:   searchType,
		},
	}
	query := &typesV8.Query{
		Bool: &typesV8.BoolQuery{
			Must: []typesV8.Query{kwQuery},
		},
	}
	if len(opts.RepoIDs) > 0 {
		repoIDs := make([]typesV8.FieldValue, 0, len(opts.RepoIDs))
		for _, repoID := range opts.RepoIDs {
			repoIDs = append(repoIDs, typesV8.FieldValue(repoID))
		}
		repoQuery := typesV8.Query{
			Terms: &typesV8.TermsQuery{
				TermsQuery: map[string]typesV8.TermsQueryField{
					"repo_id": repoIDs,
				},
			},
		}
		query.Bool.Must = append(query.Bool.Must, repoQuery)
	}

	var (
		start, pageSize = opts.GetSkipTake()
		kw              = "<em>" + opts.Keyword + "</em>"
		aggregation     = map[string]typesV8.Aggregations{
			"language": {
				Terms: &typesV8.TermsAggregation{
					Field: someV8.String("language"),
					Size:  someV8.Int(10),
					Order: map[string]sortorderV8.SortOrder{
						"_count": sortorderV8.Desc,
					},
				},
			},
		}
	)

	if len(opts.Language) == 0 {
		searchResult, err := b.inner.Client.Search().
			Index(b.inner.VersionedIndexName()).
			Aggregations(aggregation).
			Query(query).
			Highlight(
				&typesV8.Highlight{
					Fields: map[string]typesV8.HighlightField{
						"content": {
							NumberOfFragments: someV8.Int(0), // return all highting content on fragments
							Type:              &highlightertypeV8.Fvh,
						},
					},
				},
			).
			Sort("repo_id", true).
			From(start).Size(pageSize).
			Do(ctx)
		if err != nil {
			return 0, nil, nil, err
		}

		return convertResultV8(searchResult, kw, pageSize)
	}

	langQuery := typesV8.Query{
		Match: map[string]typesV8.MatchQuery{
			"language": {
				Query: opts.Language,
			},
		},
	}
	countResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Aggregations(aggregation).
		Query(query).
		Size(0). // We only need stats information
		Do(ctx)
	if err != nil {
		return 0, nil, nil, err
	}

	query.Bool.Must = append(query.Bool.Must, langQuery)
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Query(query).
		Highlight(
			&typesV8.Highlight{
				Fields: map[string]typesV8.HighlightField{
					"content": {
						NumberOfFragments: someV8.Int(0), // return all highting content on fragments
						Type:              &highlightertypeV8.Fvh,
					},
				},
			},
		).
		Sort("repo_id", true).
		From(start).Size(pageSize).
		Do(ctx)
	if err != nil {
		return 0, nil, nil, err
	}

	total, hits, _, err := convertResultV8(searchResult, kw, pageSize)

	return total, hits, extractAggregates(countResult), err
}
