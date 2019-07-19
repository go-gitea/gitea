// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package codes

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/olivere/elastic.v5"
)

var (
	_Indexer = &ElesticSearchIndexer{}
)

// ElesticSearchIndexer implements Indexer interface
type ElesticSearchIndexer struct {
	client      *elastic.Client
	indexerName string
}

// NewElesticIndexer creates a new elestic search indexer
func NewElesticIndexer(url, indexerName string) (*ElesticSearchIndexer, error) {
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

// Init will initialize the indexer
func (e *ElesticSearchIndexer) Init() (bool, error) {
	ctx := context.Background()
	exists, err := e.client.IndexExists(e.indexerName).Do(ctx)
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
						"file_path":{
							"type":"string"
						},
						"content":{
							"type":"string"
						}
					}
				}
			}
		}`

		createIndex, err := e.client.CreateIndex(e.indexerName).BodyString(mapping).Do(ctx)
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
func (e *ElesticSearchIndexer) Index(repos []*IndexerData) error {
	for _, repo := range repos {
		//tweet1 := Tweet{User: "olivere", Message: "Take Five", Retweets: 0}
		_, err := e.client.Index().
			Index(e.indexerName).
			Type("indexer_data").
			Id(fmt.Sprintf("%s", repo.RepoID)).
			BodyJson(repo).
			Do(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes indexes by ids
func (e *ElesticSearchIndexer) Delete(ids ...int64) error {
	for _, id := range ids {
		_, err := e.client.Delete().
			Index(e.indexerName).
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
func (e *ElesticSearchIndexer) Search(repoIDs []int64, keyword string, page, pageSize int) (*SearchResult, error) {
	// wrong code for code indexing (TODO)
	termQuery := elastic.NewTermQuery("title", keyword)
	searchResult, err := e.client.Search().
		Index(e.indexerName).    // search in index "twitter"
		Query(termQuery).        // specify the query
		Sort("id", true).        // sort by "user" field, ascending
		From(page).Size(pageSize). // take documents 0-9
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
