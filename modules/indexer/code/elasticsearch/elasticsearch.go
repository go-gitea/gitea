// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"
	"unicode/utf8"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/analyze"
	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer/code/internal"
	es "gitea.dev/modules/indexer/internal/elasticsearch"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/go-enry/go-enry/v2"
	"golang.org/x/text/unicode/norm"
)

const esRepoIndexerLatestVersion = 5

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	*es.Indexer
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
					"filter": [
						"lowercase"
					]
				},
				"content_short_analyzer": {
					"tokenizer": "content_short_tokenizer",
					"filter": [
						"lowercase"
					]
				}
			},
			"tokenizer": {
				"content_tokenizer": {
					"type": "ngram",
					"min_gram": 3,
					"max_gram": 3
				},
				"content_short_tokenizer": {
					"type": "ngram",
					"min_gram": 1,
					"max_gram": 2
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
				"index": true,
				"analyzer": "content_analyzer",
				"fields": {
					"short": {
						"type": "text",
						"analyzer": "content_short_analyzer",
						"index_options": "docs",
						"norms": false
					},
					"path": {
						"type": "keyword"
					}
				}
			},
			"content": {
				"type": "text",
				"index": true,
				"analyzer": "content_analyzer",
				"fields": {
					"short": {
						"type": "text",
						"analyzer": "content_short_analyzer",
						"index_options": "docs",
						"norms": false
					}
				}
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

func (b *Indexer) addUpdate(catFileBatch git.CatFileBatch, sha string, update internal.FileUpdate, repo *repo_model.Repository) (es.BulkOp, error) {
	fileContents, searchable, err := internal.ReadFile(catFileBatch, update)
	if err != nil {
		return es.BulkOp{}, err
	}
	if !searchable {
		return es.DeleteOp(internal.FilenameIndexerID(repo.ID, update.Filename)), nil
	}
	return es.IndexOp(internal.FilenameIndexerID(repo.ID, update.Filename), map[string]any{
		"repo_id":    repo.ID,
		"filename":   norm.NFC.String(update.Filename),
		"content":    string(fileContents),
		"commit_id":  sha,
		"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
		"updated_at": timeutil.TimeStampNow(),
	}), nil
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	ops := make([]es.BulkOp, 0)
	if len(changes.Updates) > 0 {
		batch, err := git.NewBatch(ctx, repo)
		if err != nil {
			return err
		}
		defer batch.Close()

		for _, update := range changes.Updates {
			operation, err := b.addUpdate(batch, sha, update, repo)
			if err != nil {
				return err
			}
			ops = append(ops, operation)
		}
	}

	for _, filename := range changes.RemovedFilenames {
		ops = append(ops, es.DeleteOp(internal.FilenameIndexerID(repo.ID, filename)))
	}

	const batchSize = 50
	for i := 0; i < len(ops); i += batchSize {
		if err := b.Bulk(ctx, ops[i:min(i+batchSize, len(ops))]); err != nil {
			return err
		}
	}
	return nil
}

// Delete entries by repoId
func (b *Indexer) Delete(ctx context.Context, repoID int64) error {
	if err := b.DeleteByQuery(ctx, es.TermsQuery("repo_id", repoID)); err != nil {
		// Maybe there is a conflict during the delete operation, so we should retry after a refresh
		log.Warn("Deletion of entries of repo %v within index %v was erroneous: %v. Trying to refresh index before trying again", repoID, b.VersionedIndexName(), err)
		if err := b.Refresh(ctx); err != nil {
			return err
		}
		if err := b.DeleteByQuery(ctx, es.TermsQuery("repo_id", repoID)); err != nil {
			log.Error("Could not delete entries of repo %v within index %v", repoID, b.VersionedIndexName())
			return err
		}
	}
	return nil
}

func convertResult(searchResult *es.SearchResponse, terms []string, pageSize int) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	matcher := internal.LiteralMatcher(terms)
	hits := make([]*internal.SearchResult, 0, pageSize)
	for _, hit := range searchResult.Hits {
		repoID, fileName := internal.ParseIndexerID(hit.ID)
		res := make(map[string]any)
		if err := json.Unmarshal(hit.Source, &res); err != nil {
			return 0, nil, nil, err
		}

		content, okContent := res["content"].(string)
		language, okLanguage := res["language"].(string)
		commitID, okCommitID := res["commit_id"].(string)
		updatedAt, okUpdatedAt := res["updated_at"].(float64)
		if !okContent || !okLanguage || !okCommitID || !okUpdatedAt {
			setting.PanicInDevOrTesting("unexpected field types in search hit %q: %s", hit.ID, string(hit.Source))
			return 0, nil, nil, fmt.Errorf("unexpected field types in search hit %q", hit.ID)
		}

		result := &internal.SearchResult{RepoID: repoID, Filename: fileName, Content: content, CommitID: commitID, Language: language, UpdatedUnix: timeutil.TimeStamp(updatedAt)}
		matcher.MatchResult(result)
		hits = append(hits, result)
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

func gramQuery(term, field string) es.Query {
	if utf8.RuneCountInString(term) < 3 {
		return es.NewMultiMatchQuery(term, field+".short").Operator("and")
	}
	return es.MatchPhraseQuery(field, term)
}

// Search searches for codes and language stats by given conditions.
func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	terms := opts.Terms
	if len(terms) == 0 {
		return 0, nil, nil, nil
	}
	query := es.NewBoolQuery()
	for _, term := range terms {
		query.Must(es.NewBoolQuery().Should(gramQuery(term, "content"), gramQuery(norm.NFC.String(term), "filename")))
	}

	if len(opts.RepoIDs) > 0 {
		query.Must(es.TermsQuery("repo_id", es.ToAnySlice(opts.RepoIDs)...))
	}
	if opts.Path != "" {
		query.Must(es.PrefixQuery("filename.path", norm.NFC.String(opts.Path)+"/"))
	}

	start, pageSize := opts.GetSkipTake()
	languageAggs := map[string]any{
		"language": map[string]any{
			"terms": map[string]any{
				"field": "language",
				"size":  10,
				"order": map[string]any{"_count": "desc"},
			},
		},
	}
	sort := []es.SortField{
		{Field: "_score", Desc: true},
		{Field: "updated_at", Desc: false},
	}

	var languages []*internal.SearchResultLanguages
	if opts.Language != "" {
		countResp, err := b.Indexer.Search(ctx, es.SearchRequest{Query: query, Size: 0, TrackTotal: true, Aggregations: languageAggs})
		if err != nil {
			return 0, nil, nil, err
		}
		languages = extractAggs(countResp)
		query.Must(es.MatchQuery("language", opts.Language))
		languageAggs = nil
	}
	resp, err := b.Indexer.Search(ctx, es.SearchRequest{
		Query: query, Sort: sort, From: start, Size: pageSize, TrackTotal: true, Aggregations: languageAggs,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	total, hits, resultLanguages, err := convertResult(resp, terms, pageSize)
	if opts.Language != "" {
		resultLanguages = languages
	}
	return total, hits, resultLanguages, err
}
