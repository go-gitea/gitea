// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"context"
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
				},
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

func (b *ElasticSearchIndexer) addUpdate(sha string, update fileUpdate, repo *models.Repository, reqs []elastic.BulkableRequest) error {
	stdout, err := git.NewCommand("cat-file", "-s", update.BlobSha).
		RunInDir(repo.RepoPath())
	if err != nil {
		return err
	}
	if size, err := strconv.Atoi(strings.TrimSpace(stdout)); err != nil {
		return fmt.Errorf("Misformatted git cat-file output: %v", err)
	} else if int64(size) > setting.Indexer.MaxIndexerFileSize {
		return b.addDelete(update.Filename, repo, reqs)
	}

	fileContents, err := git.NewCommand("cat-file", "blob", update.BlobSha).
		RunInDirBytes(repo.RepoPath())
	if err != nil {
		return err
	} else if !base.IsTextFile(fileContents) {
		// FIXME: UTF-16 files will probably fail here
		return nil
	}

	id := filenameIndexerID(repo.ID, update.Filename)

	reqs = append(reqs, elastic.NewBulkIndexRequest().
		Index(b.indexerName).
		Id(id).
		Doc(map[string]interface{}{
			"repo_id":    repo.ID,
			"content":    string(charset.ToUTF8DropErrors(fileContents)),
			"commit_id":  sha,
			"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
			"updated_at": time.Now().UTC(),
		}))

	return nil
}

func (b *ElasticSearchIndexer) addDelete(filename string, repo *models.Repository, reqs []elastic.BulkableRequest) error {
	id := filenameIndexerID(repo.ID, filename)
	reqs = append(reqs,
		elastic.NewBulkDeleteRequest().
			Index(b.indexerName).
			Id(id),
	)
	return nil
}

// Index will save the index data
func (b *ElasticSearchIndexer) Index(repo *models.Repository, sha string, changes *repoChanges) error {
	reqs := make([]elastic.BulkableRequest, 0)
	for _, update := range changes.Updates {
		if err := b.addUpdate(sha, update, repo, reqs); err != nil {
			return err
		}
	}
	for _, filename := range changes.RemovedFilenames {
		if err := b.addDelete(filename, repo, reqs); err != nil {
			return err
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
	_, err := b.client.Delete().
		Index(b.indexerName).
		Query(elastic.NewTermsQuery("repo_id", repoID)).
		Do(context.Background())
	return err
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
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
	searchResult, err := b.client.Search().
		Index(b.indexerName).
		Query(query).
		Sort("id", true).
		From(page * pageSize).Size(pageSize).
		Do(context.Background())
	if err != nil {
		return 0, nil, nil, err
	}

	hits := make([]*SearchResult, 0, pageSize)
	for _, hit := range searchResult.Hits.Hits {
		var startIndex, endIndex int = -1, -1
		/*for _, locations := range hit.Fields["Content"] {
			location := locations[0]
			locationStart := int(location.Start)
			locationEnd := int(location.End)
			if startIndex < 0 || locationStart < startIndex {
				startIndex = locationStart
			}
			if endIndex < 0 || locationEnd > endIndex {
				endIndex = locationEnd
			}
		}*/
		repoID, fileName := parseIndexerID(hit.Id)
		hits = append(hits, &SearchResult{
			RepoID:     repoID,
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Filename:   fileName,
			Content:    hit.Fields["content"].(string),
		})
	}

	return searchResult.TotalHits(), hits, nil, nil
}

// Close implements indexer
func (b *ElasticSearchIndexer) Close() {}
