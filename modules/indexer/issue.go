// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	"github.com/blevesearch/bleve/analysis/tokenizer/unicode"
	"github.com/ethantkoenig/rupture"
)

// issueIndexer (thread-safe) index for searching issues
var issueIndexer bleve.Index

const (
	issueIndexerAnalyzer = "issueIndexer"
	issueIndexerDocType  = "issueIndexerDocType"

	issueIndexerLatestVersion = 1
)

// IssueIndexerData data stored in the issue indexer
type IssueIndexerData struct {
	RepoID   int64
	Title    string
	Content  string
	Comments []string
}

// Type returns the document type, for bleve's mapping.Classifier interface.
func (i *IssueIndexerData) Type() string {
	return issueIndexerDocType
}

// IssueIndexerUpdate an update to the issue indexer
type IssueIndexerUpdate struct {
	IssueID int64
	Data    *IssueIndexerData
}

// AddToFlushingBatch adds the update to the given flushing batch.
func (i IssueIndexerUpdate) AddToFlushingBatch(batch rupture.FlushingBatch) error {
	return batch.Index(indexerID(i.IssueID), i.Data)
}

// InitIssueIndexer initialize issue indexer
func InitIssueIndexer(populateIndexer func() error) {
	var err error
	issueIndexer, err = openIndexer(setting.Indexer.IssuePath, issueIndexerLatestVersion)
	if err != nil {
		log.Fatal(4, "InitIssueIndexer: %v", err)
	}
	if issueIndexer != nil {
		return
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

	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Store = false
	textFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("Title", textFieldMapping)
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)
	docMapping.AddFieldMappingsAt("Comments", textFieldMapping)

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
	mapping.AddDocumentMapping(issueIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())

	var err error
	issueIndexer, err = bleve.New(setting.Indexer.IssuePath, mapping)
	return err
}

// IssueIndexerBatch batch to add updates to
func IssueIndexerBatch() rupture.FlushingBatch {
	return rupture.NewFlushingBatch(issueIndexer, maxBatchSize)
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

	result, err := issueIndexer.Search(search)
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
