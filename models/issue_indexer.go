// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/simple"
	"github.com/blevesearch/bleve/search/query"
)

// issueIndexerUpdateQueue queue of issues that need to be updated in the issues
// indexer
var issueIndexerUpdateQueue chan *Issue

// issueIndexer (thread-safe) index for searching issues
var issueIndexer bleve.Index

// issueIndexerData data stored in the issue indexer
type issueIndexerData struct {
	ID     int64
	RepoID int64

	Title   string
	Content string
}

// numericQuery an numeric-equality query for the given value and field
func numericQuery(value int64, field string) *query.NumericRangeQuery {
	f := float64(value)
	tru := true
	q := bleve.NewNumericRangeInclusiveQuery(&f, &f, &tru, &tru)
	q.SetField(field)
	return q
}

// SearchIssuesByKeyword searches for issues by given conditions.
// Returns the matching issue IDs
func SearchIssuesByKeyword(repoID int64, keyword string) ([]int64, error) {
	terms := strings.Fields(strings.ToLower(keyword))
	indexerQuery := bleve.NewConjunctionQuery(
		numericQuery(repoID, "RepoID"),
		bleve.NewDisjunctionQuery(
			bleve.NewPhraseQuery(terms, "Title"),
			bleve.NewPhraseQuery(terms, "Content"),
		))
	search := bleve.NewSearchRequestOptions(indexerQuery, 2147483647, 0, false)
	search.Fields = []string{"ID"}

	result, err := issueIndexer.Search(search)
	if err != nil {
		return nil, err
	}

	issueIDs := make([]int64, len(result.Hits))
	for i, hit := range result.Hits {
		issueIDs[i] = int64(hit.Fields["ID"].(float64))
	}
	return issueIDs, nil
}

// InitIssueIndexer initialize issue indexer
func InitIssueIndexer() {
	_, err := os.Stat(setting.Indexer.IssuePath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = createIssueIndexer(); err != nil {
				log.Fatal(4, "CreateIssuesIndexer: %v", err)
			}
			if err = populateIssueIndexer(); err != nil {
				log.Fatal(4, "PopulateIssuesIndex: %v", err)
			}
		} else {
			log.Fatal(4, "InitIssuesIndexer: %v", err)
		}
	} else {
		issueIndexer, err = bleve.Open(setting.Indexer.IssuePath)
		if err != nil {
			log.Fatal(4, "InitIssuesIndexer, open index: %v", err)
		}
	}
	issueIndexerUpdateQueue = make(chan *Issue, setting.Indexer.UpdateQueueLength)
	go processIssueIndexerUpdateQueue()
	// TODO close issueIndexer when Gitea closes
}

// createIssueIndexer create an issue indexer if one does not already exist
func createIssueIndexer() error {
	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	docMapping.AddFieldMappingsAt("ID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("RepoID", bleve.NewNumericFieldMapping())

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = simple.Name
	docMapping.AddFieldMappingsAt("Title", textFieldMapping)
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)

	mapping.AddDocumentMapping("issues", docMapping)

	var err error
	issueIndexer, err = bleve.New(setting.Indexer.IssuePath, mapping)
	return err
}

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer() error {
	for page := 1; ; page++ {
		repos, _, err := Repositories(&SearchRepoOptions{
			Page:     page,
			PageSize: 10,
		})
		if err != nil {
			return fmt.Errorf("Repositories: %v", err)
		}
		if len(repos) == 0 {
			return nil
		}
		batch := issueIndexer.NewBatch()
		for _, repo := range repos {
			issues, err := Issues(&IssuesOptions{
				RepoID:   repo.ID,
				IsClosed: util.OptionalBoolNone,
				IsPull:   util.OptionalBoolNone,
			})
			if err != nil {
				return fmt.Errorf("Issues: %v", err)
			}
			for _, issue := range issues {
				err = batch.Index(issue.indexUID(), issue.issueData())
				if err != nil {
					return fmt.Errorf("batch.Index: %v", err)
				}
			}
		}
		if err = issueIndexer.Batch(batch); err != nil {
			return fmt.Errorf("index.Batch: %v", err)
		}
	}
}

func processIssueIndexerUpdateQueue() {
	for {
		select {
		case issue := <-issueIndexerUpdateQueue:
			if err := issueIndexer.Index(issue.indexUID(), issue.issueData()); err != nil {
				log.Error(4, "issuesIndexer.Index: %v", err)
			}
		}
	}
}

// indexUID a unique identifier for an issue used in full-text indices
func (issue *Issue) indexUID() string {
	return strconv.FormatInt(issue.ID, 36)
}

func (issue *Issue) issueData() *issueIndexerData {
	return &issueIndexerData{
		ID:      issue.ID,
		RepoID:  issue.RepoID,
		Title:   issue.Title,
		Content: issue.Content,
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(issue *Issue) {
	go func() {
		issueIndexerUpdateQueue <- issue
	}()
}
