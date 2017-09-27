// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"os"
	"strconv"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	"github.com/blevesearch/bleve/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/index/upsidedown"
)

// issueIndexer (thread-safe) index for searching issues
var issueIndexer bleve.Index

// IssueIndexerData data stored in the issue indexer
type IssueIndexerData struct {
	RepoID     int64
	Title      string
	Content    string
	Comments   []string
	IssueIndex string
}

// IssueIndexerUpdate an update to the issue indexer
type IssueIndexerUpdate struct {
	IssueID int64
	Data    *IssueIndexerData
}

func (update IssueIndexerUpdate) addToBatch(batch *bleve.Batch) error {
	return batch.Index(indexerID(update.IssueID), update.Data)
}

const issueIndexerAnalyzer = "issueIndexer"

// InitIssueIndexer initialize issue indexer
func InitIssueIndexer(populateIndexer func() error) {
	_, err := os.Stat(setting.Indexer.IssuePath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(4, "InitIssueIndexer: %v", err)
	} else if err == nil {
		issueIndexer, err = bleve.Open(setting.Indexer.IssuePath)
		if err == nil {
			return
		} else if err != upsidedown.IncompatibleVersion {
			log.Fatal(4, "InitIssueIndexer, open index: %v", err)
		}
		log.Warn("Incompatible bleve version, deleting and recreating issue indexer")
		if err = os.RemoveAll(setting.Indexer.IssuePath); err != nil {
			log.Fatal(4, "InitIssueIndexer: remove index, %v", err)
		}
	}
	if err = createIssueIndexer(); err != nil {
		log.Fatal(4, "InitIssuesIndexer: create index, %v", err)
	}
	if err = populateIndexer(); err != nil {
		log.Fatal(4, "InitIssueIndexer: populate index, %v", err)
	}
}

// createIssueIndexer create an issue indexer if one does not already exist
func createIssueIndexer() error {
	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	docMapping.AddFieldMappingsAt("RepoID", bleve.NewNumericFieldMapping())

	textFieldMapping := bleve.NewTextFieldMapping()
	docMapping.AddFieldMappingsAt("Title", textFieldMapping)
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)
	docMapping.AddFieldMappingsAt("Comments", textFieldMapping)
	docMapping.AddFieldMappingsAt("IssueIndex", textFieldMapping)

	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return err
	} else if err = mapping.AddCustomAnalyzer(issueIndexerAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, lowercase.Name},
	}); err != nil {
		return err
	}

	mapping.DefaultAnalyzer = issueIndexerAnalyzer
	mapping.AddDocumentMapping("issues", docMapping)

	var err error
	issueIndexer, err = bleve.New(setting.Indexer.IssuePath, mapping)
	return err
}

// IssueIndexerBatch batch to add updates to
func IssueIndexerBatch() *Batch {
	return &Batch{
		batch: issueIndexer.NewBatch(),
		index: issueIndexer,
	}
}

func searchIssues(req *bleve.SearchRequest) ([]int64, error) {
	result, err := issueIndexer.Search(req)
	if err != nil {
		return nil, err
	}

	issueIDs := make([]int64, len(result.Hits))
	for i, hit := range result.Hits {
		issueIDs[i], err = idOfIndexerID(hit.ID)
		if err != nil {
			return nil, err
		}
	}

	return issueIDs, nil
}

// SearchIssuesByKeyword searches for issues by given conditions.
// Returns the matching issue IDs
func SearchIssuesByKeyword(repoID int64, keyword string) ([]int64, error) {
	indexerQuery := bleve.NewConjunctionQuery(
		numericEqualityQuery(repoID, "RepoID"),
		bleve.NewDisjunctionQuery(
			newMatchPhraseQuery(keyword, "Title", issueIndexerAnalyzer),
			newMatchPhraseQuery(keyword, "Content", issueIndexerAnalyzer),
			newMatchPhraseQuery(keyword, "Comments", issueIndexerAnalyzer),
		))
	search := bleve.NewSearchRequestOptions(indexerQuery, 2147483647, 0, false)

	return searchIssues(search)
}

// SearchIssuesByIndex searches for issues by given conditions.
// Returns the matching issue IDs
func SearchIssuesByIndex(repoID, issueIndex int64) ([]int64, error) {
	indexerQuery := bleve.NewConjunctionQuery(
		numericEqualityQuery(repoID, "RepoID"),
		newPrefixQuery(strconv.FormatInt(issueIndex, 10), "IssueIndex"))
	search := bleve.NewSearchRequestOptions(indexerQuery, 2147483647, 0, false)

	return searchIssues(search)
}
