// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"
	"io"
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
	es "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-enry/go-enry/v2"
)

const esRepoIndexerLatestVersion = 3

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	*es.Indexer
}

func (b *Indexer) SupportedSearchModes() []indexer.SearchMode {
	return indexer.SearchModesExactWords()
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) *Indexer {
	return &Indexer{Indexer: es.NewIndexer(url, indexerName, esRepoIndexerLatestVersion, defaultMapping)}
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

func (b *Indexer) addUpdate(ctx context.Context, catFileBatch git.CatFileBatch, sha string, update internal.FileUpdate, repo *repo_model.Repository) ([]es.BulkOp, error) {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil, nil
	}

	size := update.Size
	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = gitrepo.RunCmdString(ctx, repo, gitcmd.NewCommand("cat-file", "-s").AddDynamicArguments(update.BlobSha))
		if err != nil {
			return nil, err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return nil, fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}

	id := internal.FilenameIndexerID(repo.ID, update.Filename)
	if size > setting.Indexer.MaxIndexerFileSize {
		return []es.BulkOp{es.DeleteOp(id)}, nil
	}

	info, batchReader, err := catFileBatch.QueryContent(update.BlobSha)
	if err != nil {
		return nil, err
	}

	fileContents, err := io.ReadAll(io.LimitReader(batchReader, info.Size))
	if err != nil {
		return nil, err
	} else if !typesniffer.DetectContentType(fileContents).IsText() {
		// FIXME: UTF-16 files will probably fail here
		return nil, nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return nil, err
	}

	return []es.BulkOp{es.IndexOp(id, map[string]any{
		"repo_id":    repo.ID,
		"filename":   update.Filename,
		"content":    string(charset.ToUTF8DropErrors(fileContents)),
		"commit_id":  sha,
		"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
		"updated_at": timeutil.TimeStampNow(),
	})}, nil
}

func (b *Indexer) addDelete(filename string, repo *repo_model.Repository) es.BulkOp {
	return es.DeleteOp(internal.FilenameIndexerID(repo.ID, filename))
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	ops := make([]es.BulkOp, 0)
	if len(changes.Updates) > 0 {
		batch, err := gitrepo.NewBatch(ctx, repo)
		if err != nil {
			return err
		}
		defer batch.Close()

		for _, update := range changes.Updates {
			updateOps, err := b.addUpdate(ctx, batch, sha, update, repo)
			if err != nil {
				return err
			}
			if len(updateOps) > 0 {
				ops = append(ops, updateOps...)
			}
		}
	}

	for _, filename := range changes.RemovedFilenames {
		ops = append(ops, b.addDelete(filename, repo))
	}

	if len(ops) > 0 {
		esBatchSize := 50

		for i := 0; i < len(ops); i += esBatchSize {
			if err := b.Bulk(ctx, ops[i:min(i+esBatchSize, len(ops))]); err != nil {
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
		log.Warn("Deletion of entries of repo %v within index %v was erroneous: %v. Trying to refresh index before trying again", repoID, b.VersionedIndexName(), err)
		if err := b.Refresh(ctx); err != nil {
			return err
		}
		if err := b.doDelete(ctx, repoID); err != nil {
			log.Error("Could not delete entries of repo %v within index %v", repoID, b.VersionedIndexName())
			return err
		}
	}
	return nil
}

// Delete entries by repoId
func (b *Indexer) doDelete(ctx context.Context, repoID int64) error {
	return b.DeleteByQuery(ctx, es.TermsQuery("repo_id", repoID))
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

func convertResult(searchResult *es.SearchResponse, kw string, pageSize int) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	hits := make([]*internal.SearchResult, 0, pageSize)
	for _, hit := range searchResult.Hits {
		repoID, fileName := internal.ParseIndexerID(hit.ID)
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

	return searchResult.Total, hits, extractAggs(searchResult), nil
}

func extractAggs(searchResult *es.SearchResponse) []*internal.SearchResultLanguages {
	buckets, found := searchResult.Aggregations["language"]
	if !found {
		return nil
	}
	searchResultLanguages := make([]*internal.SearchResultLanguages, 0, 10)
	for _, bucket := range buckets {
		// language is mapped as keyword so the key is always a string; if the
		// mapping ever changes, skip rather than emit an empty-language bucket.
		key, ok := bucket.Key.(string)
		if !ok {
			continue
		}
		searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
			Language: key,
			Color:    enry.GetColor(key),
			Count:    int(bucket.DocCount),
		})
	}
	return searchResultLanguages
}

// Search searches for codes and language stats by given conditions.
func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	searchMode := util.IfZero(opts.SearchMode, b.SupportedSearchModes()[0].ModeValue)
	contentQuery := es.Query(es.NewMultiMatchQuery(opts.Keyword, "content").Type(es.MultiMatchTypeBestFields).Operator("and"))
	if searchMode == indexer.SearchModeExact {
		contentQuery = es.MatchPhraseQuery("content", opts.Keyword)
	}
	kwQuery := es.NewBoolQuery().Should(
		contentQuery,
		es.NewMultiMatchQuery(opts.Keyword, "filename^10").Type(es.MultiMatchTypePhrasePrefix),
	)
	query := es.NewBoolQuery().Must(kwQuery)
	if len(opts.RepoIDs) > 0 {
		query.Must(es.TermsQuery("repo_id", es.ToAnySlice(opts.RepoIDs)...))
	}

	start, pageSize := opts.GetSkipTake()
	kw := "<em>" + opts.Keyword + "</em>"
	languageAggs := map[string]any{
		"language": map[string]any{
			"terms": map[string]any{
				"field": "language",
				"size":  10,
				"order": map[string]any{"_count": "desc"},
			},
		},
	}
	// number_of_fragments=0 returns the full highlighted content (no fragmentation).
	highlight := map[string]any{
		"fields": map[string]any{
			"content":  map[string]any{},
			"filename": map[string]any{},
		},
		"number_of_fragments": 0,
		"type":                "fvh",
	}
	sort := []es.SortField{
		{Field: "_score", Desc: true},
		{Field: "updated_at", Desc: false},
	}

	if len(opts.Language) == 0 {
		resp, err := b.Indexer.Search(ctx, es.SearchRequest{
			Query:        query,
			Sort:         sort,
			From:         start,
			Size:         pageSize,
			TrackTotal:   true,
			Aggregations: languageAggs,
			Highlight:    highlight,
		})
		if err != nil {
			return 0, nil, nil, err
		}
		return convertResult(resp, kw, pageSize)
	}

	countResp, err := b.Indexer.Search(ctx, es.SearchRequest{
		Query:        query,
		Size:         0, // stats only
		TrackTotal:   true,
		Aggregations: languageAggs,
	})
	if err != nil {
		return 0, nil, nil, err
	}

	query.Must(es.MatchQuery("language", opts.Language))
	resp, err := b.Indexer.Search(ctx, es.SearchRequest{
		Query:      query,
		Sort:       sort,
		From:       start,
		Size:       pageSize,
		TrackTotal: true,
		Highlight:  highlight,
	})
	if err != nil {
		return 0, nil, nil, err
	}

	total, hits, _, err := convertResult(resp, kw, pageSize)
	return total, hits, extractAggs(countResp), err
}
