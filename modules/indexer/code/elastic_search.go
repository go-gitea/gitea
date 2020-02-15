// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/olivere/elastic/v7"
)

var (
	_ Indexer = &ElasticSearchIndexer{}
)

// ElasticSearchIndexer implements Indexer interface
type ElasticSearchIndexer struct {
	client      *elastic.Client
	indexerName string
}

type elasticLogger struct {
	*log.Logger
}

func (l elasticLogger) Printf(format string, args ...interface{}) {
	_ = l.Logger.Log(2, l.Logger.GetLevel(), format, args...)
}

// NewElasticSearchIndexer creates a new elasticsearch indexer
func NewElasticSearchIndexer(url, indexerName string) (*ElasticSearchIndexer, bool, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10 * time.Second),
		elastic.SetGzip(false),
	}

	logger := elasticLogger{log.GetLogger(log.DEFAULT)}

	if logger.GetLevel() == log.TRACE || logger.GetLevel() == log.DEBUG {
		opts = append(opts, elastic.SetTraceLog(logger))
	} else if logger.GetLevel() == log.ERROR || logger.GetLevel() == log.CRITICAL || logger.GetLevel() == log.FATAL {
		opts = append(opts, elastic.SetErrorLog(logger))
	} else if logger.GetLevel() == log.INFO || logger.GetLevel() == log.WARN {
		opts = append(opts, elastic.SetInfoLog(logger))
	}

	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, false, err
	}

	indexer := &ElasticSearchIndexer{
		client:      client,
		indexerName: indexerName,
	}
	success, err := indexer.init()

	return indexer, success, err
}

const (
	defaultMapping = `{
		"mappings": {
			"properties": {
				"repo_id": {
					"type": "integer",
					"index": true
				},
				"content": {
					"type": "text",
					"index": true
				}
			}
		}
	}`
)

// Init will initialize the indexer
func (b *ElasticSearchIndexer) init() (bool, error) {
	ctx := context.Background()
	exists, err := b.client.IndexExists(b.indexerName).Do(ctx)
	if err != nil {
		return false, err
	}

	if !exists {
		var mapping = defaultMapping

		createIndex, err := b.client.CreateIndex(b.indexerName).BodyString(mapping).Do(ctx)
		if err != nil {
			return false, err
		}
		if !createIndex.Acknowledged {
			return false, errors.New("init failed")
		}

		return false, nil
	}
	return true, nil
}

func (b *ElasticSearchIndexer) addUpdate(sha string, update fileUpdate, repo *models.Repository) ([]elastic.BulkableRequest, error) {
	stdout, err := git.NewCommand("cat-file", "-s", update.BlobSha).
		RunInDir(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	if size, err := strconv.Atoi(strings.TrimSpace(stdout)); err != nil {
		return nil, fmt.Errorf("Misformatted git cat-file output: %v", err)
	} else if int64(size) > setting.Indexer.MaxIndexerFileSize {
		return b.addDelete(update.Filename, repo)
	}

	fileContents, err := git.NewCommand("cat-file", "blob", update.BlobSha).
		RunInDirBytes(repo.RepoPath())
	if err != nil {
		return nil, err
	} else if !base.IsTextFile(fileContents) {
		// FIXME: UTF-16 files will probably fail here
		return nil, nil
	}

	id := filenameIndexerID(repo.ID, update.Filename)

	return []elastic.BulkableRequest{
		elastic.NewBulkIndexRequest().
			Index(b.indexerName).
			Id(id).
			Doc(map[string]interface{}{
				"repo_id":    repo.ID,
				"content":    string(charset.ToUTF8DropErrors(fileContents)),
				"commit_id":  sha,
				"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
				"updated_at": time.Now().UTC(),
			}),
	}, nil
}

func (b *ElasticSearchIndexer) addDelete(filename string, repo *models.Repository) ([]elastic.BulkableRequest, error) {
	id := filenameIndexerID(repo.ID, filename)
	return []elastic.BulkableRequest{
		elastic.NewBulkDeleteRequest().
			Index(b.indexerName).
			Id(id),
	}, nil
}

// Index will save the index data
func (b *ElasticSearchIndexer) Index(repo *models.Repository, sha string, changes *repoChanges) error {
	reqs := make([]elastic.BulkableRequest, 0)
	for _, update := range changes.Updates {
		updateReqs, err := b.addUpdate(sha, update, repo)
		if err != nil {
			return err
		}
		if len(updateReqs) > 0 {
			reqs = append(reqs, updateReqs...)
		}
	}

	for _, filename := range changes.RemovedFilenames {
		delReqs, err := b.addDelete(filename, repo)
		if err != nil {
			return err
		}
		if len(delReqs) > 0 {
			reqs = append(reqs, delReqs...)
		}
	}

	if len(reqs) > 0 {
		_, err := b.client.Bulk().
			Index(b.indexerName).
			Add(reqs...).
			Do(context.Background())
		return err
	}
	return nil
}

// Delete deletes indexes by ids
func (b *ElasticSearchIndexer) Delete(repoID int64) error {
	_, err := b.client.DeleteByQuery(b.indexerName).
		Query(elastic.NewTermsQuery("repo_id", repoID)).
		Do(context.Background())
	return err
}

// Search searches for codes and language stats by given conditions.
func (b *ElasticSearchIndexer) Search(repoIDs []int64, language, keyword string, page, pageSize int) (int64, []*SearchResult, []*SearchResultLanguages, error) {
	kwQuery := elastic.NewMultiMatchQuery(keyword, "content")
	query := elastic.NewBoolQuery()
	query = query.Must(kwQuery)
	if len(repoIDs) > 0 {
		var repoStrs = make([]interface{}, 0, len(repoIDs))
		for _, repoID := range repoIDs {
			repoStrs = append(repoStrs, repoID)
		}
		repoQuery := elastic.NewTermsQuery("repo_id", repoStrs...)
		query = query.Must(repoQuery)
	}
	start := 0
	if page > 0 {
		start = (page - 1) * pageSize
	}
	searchResult, err := b.client.Search().
		Index(b.indexerName).
		Query(query).
		Highlight(elastic.NewHighlight().Field("content")).
		Sort("repo_id", true).
		From(start).Size(pageSize).
		Do(context.Background())
	if err != nil {
		return 0, nil, nil, err
	}

	var kw = "<em>" + keyword + "</em>"

	hits := make([]*SearchResult, 0, pageSize)
	for _, hit := range searchResult.Hits.Hits {
		var startIndex, endIndex int = -1, -1
		c, ok := hit.Highlight["content"]
		if ok && len(c) > 0 {
			startIndex = strings.Index(c[0], kw)
			if startIndex > -1 {
				endIndex = startIndex + len(kw)
			}
		}

		repoID, fileName := parseIndexerID(hit.Id)
		var h = SearchResult{
			RepoID:     repoID,
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Filename:   fileName,
		}

		if err := json.Unmarshal(hit.Source, &h); err != nil {
			return 0, nil, nil, err
		}

		hits = append(hits, &h)
	}

	return searchResult.TotalHits(), hits, nil, nil
}

// Close implements indexer
func (b *ElasticSearchIndexer) Close() {}
