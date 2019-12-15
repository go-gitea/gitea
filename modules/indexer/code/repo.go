// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"context"
	"os"
	"strings"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	"github.com/blevesearch/bleve/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/search/query"
	"github.com/ethantkoenig/rupture"
)

const (
	repoIndexerAnalyzer = "repoIndexerAnalyzer"
	repoIndexerDocType  = "repoIndexerDocType"

	repoIndexerLatestVersion = 4
)

type bleveIndexerHolder struct {
	index bleve.Index
	mutex sync.RWMutex
	cond  *sync.Cond
}

func newBleveIndexerHolder() *bleveIndexerHolder {
	b := &bleveIndexerHolder{}
	b.cond = sync.NewCond(b.mutex.RLocker())
	return b
}

func (r *bleveIndexerHolder) set(index bleve.Index) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.index = index
	r.cond.Broadcast()
}

func (r *bleveIndexerHolder) get() bleve.Index {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.index == nil {
		r.cond.Wait()
	}
	return r.index
}

// repoIndexer (thread-safe) index for repository contents
var indexerHolder = newBleveIndexerHolder()

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

// Type returns the document type, for bleve's mapping.Classifier interface.
func (d *RepoIndexerData) Type() string {
	return repoIndexerDocType
}

// RepoIndexerUpdate an update to the repo indexer
type RepoIndexerUpdate struct {
	Filepath string
	Op       RepoIndexerOp
	Data     *RepoIndexerData
}

// AddToFlushingBatch adds the update to the given flushing batch.
func (update RepoIndexerUpdate) AddToFlushingBatch(batch rupture.FlushingBatch) error {
	id := filenameIndexerID(update.Data.RepoID, update.Filepath)
	switch update.Op {
	case RepoIndexerOpUpdate:
		return batch.Index(id, update.Data)
	case RepoIndexerOpDelete:
		return batch.Delete(id)
	default:
		log.Error("Unrecognized repo indexer op: %d", update.Op)
	}
	return nil
}

// initRepoIndexer initialize repo indexer
func initRepoIndexer(populateIndexer func() error) {
	indexer, err := openIndexer(setting.Indexer.RepoPath, repoIndexerLatestVersion)
	if err != nil {
		log.Fatal("InitRepoIndexer %s: %v", setting.Indexer.RepoPath, err)
	}
	if indexer != nil {
		indexerHolder.set(indexer)
		closeAtTerminate()

		// Continue population from where left off
		if err = populateIndexer(); err != nil {
			log.Fatal("PopulateRepoIndex: %v", err)
		}
		return
	}

	if err = createRepoIndexer(setting.Indexer.RepoPath, repoIndexerLatestVersion); err != nil {
		log.Fatal("CreateRepoIndexer: %v", err)
	}
	closeAtTerminate()

	// if there is any existing repo indexer metadata in the DB, delete it
	// since we are starting afresh. Also, xorm requires deletes to have a
	// condition, and we want to delete everything, thus 1=1.
	if err := models.DeleteAllRecords("repo_indexer_status"); err != nil {
		log.Fatal("DeleteAllRepoIndexerStatus: %v", err)
	}

	if err = populateIndexer(); err != nil {
		log.Fatal("PopulateRepoIndex: %v", err)
	}
}

func closeAtTerminate() {
	graceful.GetManager().RunAtTerminate(context.Background(), func() {
		log.Debug("Closing repo indexer")
		indexer := indexerHolder.get()
		if indexer != nil {
			err := indexer.Close()
			if err != nil {
				log.Error("Error whilst closing the repository indexer: %v", err)
			}
		}
		log.Info("PID: %d Repository Indexer closed", os.Getpid())
	})
}

// createRepoIndexer create a repo indexer if one does not already exist
func createRepoIndexer(path string, latestVersion int) error {
	docMapping := bleve.NewDocumentMapping()
	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)

	mapping := bleve.NewIndexMapping()
	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return err
	} else if err := mapping.AddCustomAnalyzer(repoIndexerAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, lowercase.Name},
	}); err != nil {
		return err
	}
	mapping.DefaultAnalyzer = repoIndexerAnalyzer
	mapping.AddDocumentMapping(repoIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())

	indexer, err := bleve.New(path, mapping)
	if err != nil {
		return err
	}
	indexerHolder.set(indexer)

	return rupture.WriteIndexMetadata(path, &rupture.IndexMetadata{
		Version: latestVersion,
	})
}

func filenameIndexerID(repoID int64, filename string) string {
	return indexerID(repoID) + "_" + filename
}

func filenameOfIndexerID(indexerID string) string {
	index := strings.IndexByte(indexerID, '_')
	if index == -1 {
		log.Error("Unexpected ID in repo indexer: %s", indexerID)
	}
	return indexerID[index+1:]
}

// RepoIndexerBatch batch to add updates to
func RepoIndexerBatch() rupture.FlushingBatch {
	return rupture.NewFlushingBatch(indexerHolder.get(), maxBatchSize)
}

// deleteRepoFromIndexer delete all of a repo's files from indexer
func deleteRepoFromIndexer(repoID int64) error {
	query := numericEqualityQuery(repoID, "RepoID")
	searchRequest := bleve.NewSearchRequestOptions(query, 2147483647, 0, false)
	result, err := indexerHolder.get().Search(searchRequest)
	if err != nil {
		return err
	}
	batch := RepoIndexerBatch()
	for _, hit := range result.Hits {
		if err = batch.Delete(hit.ID); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// RepoSearchResult result of performing a search in a repo
type RepoSearchResult struct {
	RepoID     int64
	StartIndex int
	EndIndex   int
	Filename   string
	Content    string
}

// SearchRepoByKeyword searches for files in the specified repo.
// Returns the matching file-paths
func SearchRepoByKeyword(repoIDs []int64, keyword string, page, pageSize int) (int64, []*RepoSearchResult, error) {
	phraseQuery := bleve.NewMatchPhraseQuery(keyword)
	phraseQuery.FieldVal = "Content"
	phraseQuery.Analyzer = repoIndexerAnalyzer

	var indexerQuery query.Query
	if len(repoIDs) > 0 {
		var repoQueries = make([]query.Query, 0, len(repoIDs))
		for _, repoID := range repoIDs {
			repoQueries = append(repoQueries, numericEqualityQuery(repoID, "RepoID"))
		}

		indexerQuery = bleve.NewConjunctionQuery(
			bleve.NewDisjunctionQuery(repoQueries...),
			phraseQuery,
		)
	} else {
		indexerQuery = phraseQuery
	}

	from := (page - 1) * pageSize
	searchRequest := bleve.NewSearchRequestOptions(indexerQuery, pageSize, from, false)
	searchRequest.Fields = []string{"Content", "RepoID"}
	searchRequest.IncludeLocations = true

	result, err := indexerHolder.get().Search(searchRequest)
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
			RepoID:     int64(hit.Fields["RepoID"].(float64)),
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Filename:   filenameOfIndexerID(hit.ID),
			Content:    hit.Fields["Content"].(string),
		}
	}
	return int64(result.Total), searchResults, nil
}
