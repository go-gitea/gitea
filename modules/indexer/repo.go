// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/token/camelcase"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	"github.com/blevesearch/bleve/analysis/tokenizer/unicode"
)

const repoIndexerAnalyzer = "repoIndexerAnalyzer"

// repoIndexer (thread-safe) index for repository contents
var repoIndexer bleve.Index

// RepoIndexerOp type of operation to perform on repo indexer
type RepoIndexerOp int

const (
	// RepoIndexerOpUpdate add/update a file's contents
	RepoIndexerOpUpdate = iota

	// RepoIndexerOpDelete delete a file
	RepoIndexerOpDelete
)

// RepoIndexerData data stored in the repo indexer
type RepoIndexerData struct {
	RepoID  int64
	Content string
}

// RepoIndexerUpdate an update to the repo indexer
type RepoIndexerUpdate struct {
	Filepath string
	Op       RepoIndexerOp
	Data     *RepoIndexerData
}

func (update RepoIndexerUpdate) addToBatch(batch *bleve.Batch) error {
	id := filenameIndexerID(update.Data.RepoID, update.Filepath)
	switch update.Op {
	case RepoIndexerOpUpdate:
		return batch.Index(id, update.Data)
	case RepoIndexerOpDelete:
		batch.Delete(id)
	default:
		log.Error(4, "Unrecognized repo indexer op: %d", update.Op)
	}
	return nil
}

// InitRepoIndexer initialize repo indexer
func InitRepoIndexer(populateIndexer func() error) {
	_, err := os.Stat(setting.Indexer.RepoPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = createRepoIndexer(); err != nil {
				log.Fatal(4, "CreateRepoIndexer: %v", err)
			}
			if err = populateIndexer(); err != nil {
				log.Fatal(4, "PopulateRepoIndex: %v", err)
			}
		} else {
			log.Fatal(4, "InitRepoIndexer: %v", err)
		}
	} else {
		repoIndexer, err = bleve.Open(setting.Indexer.RepoPath)
		if err != nil {
			log.Fatal(4, "InitRepoIndexer, open index: %v", err)
		}
	}
}

// createRepoIndexer create a repo indexer if one does not already exist
func createRepoIndexer() error {
	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("RepoID", bleve.NewNumericFieldMapping())

	textFieldMapping := bleve.NewTextFieldMapping()
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)

	mapping := bleve.NewIndexMapping()
	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return err
	} else if err := mapping.AddCustomAnalyzer(repoIndexerAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, camelcase.Name, lowercase.Name},
	}); err != nil {
		return err
	}
	mapping.DefaultAnalyzer = repoIndexerAnalyzer
	mapping.AddDocumentMapping("repo", docMapping)
	var err error
	repoIndexer, err = bleve.New(setting.Indexer.RepoPath, mapping)
	return err
}

func filenameIndexerID(repoID int64, filename string) string {
	return indexerID(repoID) + "_" + filename
}

func filenameOfIndexerID(indexerID string) string {
	index := strings.IndexByte(indexerID, '_')
	if index == -1 {
		log.Error(4, "Unexpected ID in repo indexer: %s", indexerID)
	}
	return indexerID[index+1:]
}

// RepoIndexerBatch batch to add updates to
func RepoIndexerBatch() *Batch {
	return &Batch{
		batch: repoIndexer.NewBatch(),
		index: repoIndexer,
	}
}

// DeleteRepoFromIndexer delete all of a repo's files from indexer
func DeleteRepoFromIndexer(repoID int64) error {
	query := numericEqualityQuery(repoID, "RepoID")
	searchRequest := bleve.NewSearchRequestOptions(query, 2147483647, 0, false)
	result, err := repoIndexer.Search(searchRequest)
	if err != nil {
		return err
	}
	batch := RepoIndexerBatch()
	for _, hit := range result.Hits {
		batch.batch.Delete(hit.ID)
		if err = batch.flushIfFull(); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// RepoSearchResult result of performing a search in a repo
type RepoSearchResult struct {
	StartIndex int
	EndIndex   int
	Filename   string
	Content    string
}

// SearchRepoByKeyword searches for files in the specified repo.
// Returns the matching file-paths
func SearchRepoByKeyword(repoID int64, keyword string, page, pageSize int) (int64, []*RepoSearchResult, error) {
	phraseQuery := bleve.NewMatchPhraseQuery(keyword)
	phraseQuery.FieldVal = "Content"
	phraseQuery.Analyzer = repoIndexerAnalyzer
	indexerQuery := bleve.NewConjunctionQuery(
		numericEqualityQuery(repoID, "RepoID"),
		phraseQuery,
	)
	from := (page - 1) * pageSize
	searchRequest := bleve.NewSearchRequestOptions(indexerQuery, pageSize, from, false)
	searchRequest.Fields = []string{"Content"}
	searchRequest.IncludeLocations = true

	result, err := repoIndexer.Search(searchRequest)
	if err != nil {
		return 0, nil, err
	}

	searchResults := make([]*RepoSearchResult, len(result.Hits))
	for i, hit := range result.Hits {
		var startIndex, endIndex int = -1, -1
		for _, locations := range hit.Locations["Content"] {
			location := locations[0]
			locationStart := int(location.Start)
			locationEnd := int(location.End)
			if startIndex < 0 || locationStart < startIndex {
				startIndex = locationStart
			}
			if endIndex < 0 || locationEnd > endIndex {
				endIndex = locationEnd
			}
		}
		searchResults[i] = &RepoSearchResult{
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Filename:   filenameOfIndexerID(hit.ID),
			Content:    hit.Fields["Content"].(string),
		}
	}
	return int64(result.Total), searchResults, nil
}
