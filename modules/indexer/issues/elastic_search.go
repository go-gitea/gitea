// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/olivere/elastic.v5"
)

var (
	_ Indexer = &ElesticSearchIndexer{}
)

// ElesticSearchIndexer implements Indexer interface
type ElesticSearchIndexer struct {
	client      *elastic.Client
	indexerName string
}

// NewElesticSearchIndexer creates a new elestic search indexer
func NewElesticSearchIndexer(url, indexerName string) (*ElesticSearchIndexer, error) {
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetGzip(true),
		elastic.SetErrorLog(log.New(os.Stderr, "ELASTIC ", log.LstdFlags)),
		elastic.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
	)

	if err != nil {
		return nil, err
	}

	return &ElesticSearchIndexer{
		client:      client,
		indexerName: indexerName,
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
							"type":"number"
						},
						"title":{
							"type":"string"
						},
						"content":{
							"type":"string"
						},
						"comment":{
							"type":"string"
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
		}

		return true, nil
	}
	return false, nil
}

// Index will save the index data
func (b *ElesticSearchIndexer) Index(issues []*IndexerData) error {
	for _, issue := range issues {
		//tweet1 := Tweet{User: "olivere", Message: "Take Five", Retweets: 0}
		_, err := b.client.Index().
			Index(b.indexerName).
			Type("indexer_data").
			Id(fmt.Sprintf("%d", issue.ID)).
			BodyJson(issue).
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
			Type("tweindexer_dataet").
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
	termQuery := elastic.NewTermQuery("title", keyword)
	searchResult, err := b.client.Search().
		Index(b.indexerName).    // search in index "twitter"
		Query(termQuery).        // specify the query
		Sort("id", true).        // sort by "user" field, ascending
		From(start).Size(limit). // take documents 0-9
		Pretty(true).            // pretty print request and response JSON
		Do(context.Background()) // execute
	if err != nil {
		return nil, err
	}
	fmt.Println(searchResult)

	return &SearchResult{
		Hits: []Match{},
	}, nil
}
