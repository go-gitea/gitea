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
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/typesniffer"

	"github.com/go-enry/go-enry/v2"
	"github.com/olivere/elastic/v7"
)

const (
	esRepoIndexerLatestVersion = 3
	// multi-match-types, currently only 2 types are used
	// Reference: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-multi-match-query.html#multi-match-types
	esMultiMatchTypeBestFields   = "best_fields"
	esMultiMatchTypePhrasePrefix = "phrase_prefix"
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_elasticsearch.Indexer
	indexer_internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) *Indexer {
	inner := inner_elasticsearch.NewIndexer(url, indexerName, esRepoIndexerLatestVersion, defaultMapping)
	indexer := &Indexer{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

const (
	defaultMapping = `{
		"settings": {
    		"analysis": {
      			"analyzer": {
					"content_analyzer": {
						"tokenizer": "content_tokenizer",
						"filter" : ["lowercase"]
					},
        			"filename_path_analyzer": {
          				"tokenizer": "path_tokenizer"
        			},
        			"reversed_filename_path_analyzer": {
          				"tokenizer": "reversed_path_tokenizer"
        			}
      			},
				"tokenizer": {
					"content_tokenizer": {
						"type": "simple_pattern_split",
						"pattern": "[^a-zA-Z0-9]"
					},
					"path_tokenizer": {
						"type": "path_hierarchy",
						"delimiter": "/"
					},
					"reversed_path_tokenizer": {
						"type": "path_hierarchy",
						"delimiter": "/",
						"reverse": true
					}
				}
			}
  		},
		"mappings": {
			"properties": {
				"repo_id": {
					"type": "long",
					"index": true
				},
				"filename": {
					"type": "text",
					"term_vector": "with_positions_offsets",
					"index": true,
					"fields": {
         		  		"path": {
            				"type": "text",
            				"analyzer": "reversed_filename_path_analyzer"
						},
          				"path_reversed": {
            				"type": "text",
            				"analyzer": "filename_path_analyzer"
          				}
        			}
				},
				"content": {
					"type": "text",
					"term_vector": "with_positions_offsets",
					"index": true,
					"analyzer": "content_analyzer"
				},
				"commit_id": {
					"type": "keyword",
					"index": true
				},
				"language": {
					"type": "keyword",
					"index": true
				},
				"updated_at": {
					"type": "long",
					"index": true
				}
			}
		}
	}`
)

func (b *Indexer) addUpdate(ctx context.Context, batchWriter git.WriteCloserError, batchReader *bufio.Reader, sha string, update internal.FileUpdate, repo *repo_model.Repository) ([]elastic.BulkableRequest, error) {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil, nil
	}

	size := update.Size
	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = git.NewCommand(ctx, "cat-file", "-s").AddDynamicArguments(update.BlobSha).RunStdString(&git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			return nil, err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return nil, fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}

	if size > setting.Indexer.MaxIndexerFileSize {
		return []elastic.BulkableRequest{b.addDelete(update.Filename, repo)}, nil
	}

	if _, err := batchWriter.Write([]byte(update.BlobSha + "\n")); err != nil {
		return nil, err
	}

	_, _, size, err = git.ReadBatchLine(batchReader)
	if err != nil {
		return nil, err
	}

	fileContents, err := io.ReadAll(io.LimitReader(batchReader, size))
	if err != nil {
		return nil, err
	} else if !typesniffer.DetectContentType(fileContents).IsText() {
		// FIXME: UTF-16 files will probably fail here
		return nil, nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return nil, err
	}
	id := internal.FilenameIndexerID(repo.ID, update.Filename)

	return []elastic.BulkableRequest{
		elastic.NewBulkIndexRequest().
			Index(b.inner.VersionedIndexName()).
			Id(id).
			Doc(map[string]any{
				"repo_id":    repo.ID,
				"filename":   update.Filename,
				"content":    string(charset.ToUTF8DropErrors(fileContents, charset.ConvertOpts{})),
				"commit_id":  sha,
				"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
				"updated_at": timeutil.TimeStampNow(),
			}),
	}, nil
}

func (b *Indexer) addDelete(filename string, repo *repo_model.Repository) elastic.BulkableRequest {
	id := internal.FilenameIndexerID(repo.ID, filename)
	return elastic.NewBulkDeleteRequest().
		Index(b.inner.VersionedIndexName()).
		Id(id)
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	reqs := make([]elastic.BulkableRequest, 0)
	if len(changes.Updates) > 0 {
		r, err := gitrepo.OpenRepository(ctx, repo)
		if err != nil {
			return err
		}
		defer r.Close()
		batch, err := r.NewBatch(ctx)
		if err != nil {
			return err
		}
		defer batch.Close()

		for _, update := range changes.Updates {
			updateReqs, err := b.addUpdate(ctx, batch.Writer, batch.Reader, sha, update, repo)
			if err != nil {
				return err
			}
			if len(updateReqs) > 0 {
				reqs = append(reqs, updateReqs...)
			}
		}
		batch.Close()
	}

	for _, filename := range changes.RemovedFilenames {
		reqs = append(reqs, b.addDelete(filename, repo))
	}

	if len(reqs) > 0 {
		esBatchSize := 50

		for i := 0; i < len(reqs); i += esBatchSize {
			_, err := b.inner.Client.Bulk().
				Index(b.inner.VersionedIndexName()).
				Add(reqs[i:min(i+esBatchSize, len(reqs))]...).
				Do(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete entries by repoId
func (b *Indexer) Delete(ctx context.Context, repoID int64) error {
	if err := b.doDelete(ctx, repoID); err != nil {
		// Maybe there is a conflict during the delete operation, so we should retry after a refresh
		log.Warn("Deletion of entries of repo %v within index %v was erroneus. Trying to refresh index before trying again", repoID, b.inner.VersionedIndexName(), err)
		if err := b.refreshIndex(ctx); err != nil {
			return err
		}
		if err := b.doDelete(ctx, repoID); err != nil {
			log.Error("Could not delete entries of repo %v within index %v", repoID, b.inner.VersionedIndexName())
			return err
		}
	}
	return nil
}

func (b *Indexer) refreshIndex(ctx context.Context) error {
	if _, err := b.inner.Client.Refresh(b.inner.VersionedIndexName()).Do(ctx); err != nil {
		log.Error("Error while trying to refresh index %v", b.inner.VersionedIndexName(), err)
		return err
	}

	return nil
}

// Delete entries by repoId
func (b *Indexer) doDelete(ctx context.Context, repoID int64) error {
	_, err := b.inner.Client.DeleteByQuery(b.inner.VersionedIndexName()).
		Query(elastic.NewTermsQuery("repo_id", repoID)).
		Do(ctx)
	return err
}

// contentMatchIndexPos find words positions for start and the following end on content. It will
// return the beginning position of the first start and the ending position of the
// first end following the start string.
// If not found any of the positions, it will return -1, -1.
func contentMatchIndexPos(content, start, end string) (int, int) {
	startIdx := strings.Index(content, start)
	if startIdx < 0 {
		return -1, -1
	}
	endIdx := strings.Index(content[startIdx+len(start):], end)
	if endIdx < 0 {
		return -1, -1
	}
	return startIdx, (startIdx + len(start) + endIdx + len(end)) - 9 // remove the length <em></em> since we give Content the original data
}

func convertResult(searchResult *elastic.SearchResult, kw string, pageSize int) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	hits := make([]*internal.SearchResult, 0, pageSize)
	for _, hit := range searchResult.Hits.Hits {
		repoID, fileName := internal.ParseIndexerID(hit.Id)
		res := make(map[string]any)
		if err := json.Unmarshal(hit.Source, &res); err != nil {
			return 0, nil, nil, err
		}

		// FIXME: There is no way to get the position the keyword on the content currently on the same request.
		// So we get it from content, this may made the query slower. See
		// https://discuss.elastic.co/t/fetching-position-of-keyword-in-matched-document/94291
		var startIndex, endIndex int
		if c, ok := hit.Highlight["filename"]; ok && len(c) > 0 {
			startIndex, endIndex = internal.FilenameMatchIndexPos(res["content"].(string))
		} else if c, ok := hit.Highlight["content"]; ok && len(c) > 0 {
			// FIXME: Since the highlighting content will include <em> and </em> for the keywords,
			// now we should find the positions. But how to avoid html content which contains the
			// <em> and </em> tags? If elastic search has handled that?
			startIndex, endIndex = contentMatchIndexPos(c[0], "<em>", "</em>")
			if startIndex == -1 {
				panic(fmt.Sprintf("1===%s,,,%#v,,,%s", kw, hit.Highlight, c[0]))
			}
		} else {
			panic(fmt.Sprintf("2===%#v", hit.Highlight))
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
			EndIndex:    endIndex,
			Color:       enry.GetColor(language),
		})
	}

	return searchResult.TotalHits(), hits, extractAggs(searchResult), nil
}

func extractAggs(searchResult *elastic.SearchResult) []*internal.SearchResultLanguages {
	var searchResultLanguages []*internal.SearchResultLanguages
	agg, found := searchResult.Aggregations.Terms("language")
	if found {
		searchResultLanguages = make([]*internal.SearchResultLanguages, 0, 10)

		for _, bucket := range agg.Buckets {
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
func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	searchType := esMultiMatchTypePhrasePrefix
	if opts.IsKeywordFuzzy {
		searchType = esMultiMatchTypeBestFields
	}

	kwQuery := elastic.NewBoolQuery().Should(
		elastic.NewMultiMatchQuery(opts.Keyword, "content").Type(searchType),
		elastic.NewMultiMatchQuery(opts.Keyword, "filename^10").Type(esMultiMatchTypePhrasePrefix),
	)
	query := elastic.NewBoolQuery()
	query = query.Must(kwQuery)
	if len(opts.RepoIDs) > 0 {
		repoStrs := make([]any, 0, len(opts.RepoIDs))
		for _, repoID := range opts.RepoIDs {
			repoStrs = append(repoStrs, repoID)
		}
		repoQuery := elastic.NewTermsQuery("repo_id", repoStrs...)
		query = query.Must(repoQuery)
	}

	var (
		start, pageSize = opts.GetSkipTake()
		kw              = "<em>" + opts.Keyword + "</em>"
		aggregation     = elastic.NewTermsAggregation().Field("language").Size(10).OrderByCountDesc()
	)

	if len(opts.Language) == 0 {
		searchResult, err := b.inner.Client.Search().
			Index(b.inner.VersionedIndexName()).
			Aggregation("language", aggregation).
			Query(query).
			Highlight(
				elastic.NewHighlight().
					Field("content").
					Field("filename").
					NumOfFragments(0). // return all highting content on fragments
					HighlighterType("fvh"),
			).
			Sort("_score", false).
			Sort("updated_at", true).
			From(start).Size(pageSize).
			Do(ctx)
		if err != nil {
			return 0, nil, nil, err
		}

		return convertResult(searchResult, kw, pageSize)
	}

	langQuery := elastic.NewMatchQuery("language", opts.Language)
	countResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Aggregation("language", aggregation).
		Query(query).
		Size(0). // We only need stats information
		Do(ctx)
	if err != nil {
		return 0, nil, nil, err
	}

	query = query.Must(langQuery)
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Query(query).
		Highlight(
			elastic.NewHighlight().
				Field("content").
				Field("filename").
				NumOfFragments(0). // return all highting content on fragments
				HighlighterType("fvh"),
		).
		Sort("_score", false).
		Sort("updated_at", true).
		From(start).Size(pageSize).
		Do(ctx)
	if err != nil {
		return 0, nil, nil, err
	}

	total, hits, _, err := convertResult(searchResult, kw, pageSize)

	return total, hits, extractAggs(countResult), err
}
