// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/log"

	"gopkg.in/olivere/elastic.v5"
)

var (
	_ Indexer = &ElesticSearchIndexer{}
)

// ElesticSearchIndexer implements Indexer interface
type ElesticSearchIndexer struct {
	client      *elastic.Client
	indexerName string
	typeName    string
}

type elasticLogger struct {
	*log.Logger
}

func (l elasticLogger) Printf(format string, args ...interface{}) {
	l.Logger.Log(2, l.Logger.GetLevel(), format, args...)
}

// NewElesticSearchIndexer creates a new elestic search indexer
func NewElesticSearchIndexer(url, indexerName string) (*ElesticSearchIndexer, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10 * time.Second),
		elastic.SetGzip(false),
	}

	logger := log.GetLogger("elastic")

	if logger.GetLevel() == log.TRACE || logger.GetLevel() == log.DEBUG {
		opts = append(opts, elastic.SetTraceLog(elasticLogger{logger}))
	} else if logger.GetLevel() == log.ERROR || logger.GetLevel() == log.CRITICAL || logger.GetLevel() == log.FATAL {
		opts = append(opts, elastic.SetErrorLog(elasticLogger{logger}))
	} else if logger.GetLevel() == log.INFO || logger.GetLevel() == log.WARN {
		opts = append(opts, elastic.SetInfoLog(elasticLogger{logger}))
	}

	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	return &ElesticSearchIndexer{
		client:      client,
		indexerName: indexerName,
		typeName:    "indexer_data",
	}, nil
}

// Init will initial the indexer
func (b *ElesticSearchIndexer) Init() (bool, error) {
	ctx := context.Background()
	exists, err := b.client.IndexExists(b.indexerName).Do(ctx)
	if err != nil {
		return false, err
	}

	if !exists {
		mapping := `{
			"settings":{
				"number_of_shards":1,
				"number_of_replicas":0
			},
			"mappings":{
				"indexer_data":{
					"properties":{
						"repo_id":{
							"type":"integer",
							"index": true
						},
						"title":{
							"type":"text",
							"index": true
						},
						"content":{
							"type":"text",
							"index": true
						},
						"comments":{
							"type" : "text", 
							"index": true
						}
					}
				}
			}
		}`

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

// Index will save the index data
func (b *ElesticSearchIndexer) Index(issues []*IndexerData) error {
	for _, issue := range issues {
		_, err := b.client.Index().
			Index(b.indexerName).
			Type(b.typeName).
			Id(fmt.Sprintf("%d", issue.ID)).
			BodyJson(map[string]interface{}{
				"id":       issue.ID,
				"repo_id":  issue.RepoID,
				"title":    issue.Title,
				"content":  issue.Content,
				"comments": issue.Comments,
			}).
			Do(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes indexes by ids
func (b *ElesticSearchIndexer) Delete(ids ...int64) error {
	for _, id := range ids {
		_, err := b.client.Delete().
			Index(b.indexerName).
			Type(b.typeName).
			Id(fmt.Sprintf("%d", id)).
			Do(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *ElesticSearchIndexer) Search(keyword string, repoID int64, limit, start int) (*SearchResult, error) {
	kwQuery := elastic.NewMultiMatchQuery(keyword, "title", "content", "comments")
	query := elastic.NewBoolQuery()
	query = query.Filter(kwQuery)
	if repoID > 0 {
		repoQuery := elastic.NewTermQuery("repo_id", repoID)
		query = query.Filter(repoQuery)
	}
	searchResult, err := b.client.Search().
		Index(b.indexerName).
		Type(b.typeName).
		Query(query).
		Sort("id", true).
		From(start).Size(limit).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	hits := make([]Match, 0, limit)
	for _, hit := range searchResult.Hits.Hits {
		items := make(map[string]interface{})
		err := json.Unmarshal(*hit.Source, &items)
		if err != nil {
			return nil, err
		}
		hits = append(hits, Match{
			ID:     int64(items["id"].(float64)),
			RepoID: int64(items["repo_id"].(float64)),
		})
	}

	return &SearchResult{
		Total: searchResult.TotalHits(),
		Hits:  hits,
	}, nil
}
