// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package codes

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/token/camelcase"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	"github.com/blevesearch/bleve/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/analysis/token/unique"
	"github.com/blevesearch/bleve/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/index/upsidedown"
	"github.com/blevesearch/bleve/mapping"
	"github.com/blevesearch/bleve/search/query"
	"github.com/ethantkoenig/rupture"
)

// indexerID a bleve-compatible unique identifier for an integer id
func indexerID(id int64) string {
	return strconv.FormatInt(id, 36)
}

// idOfIndexerID the integer id associated with an indexer id
func idOfIndexerID(indexerID string) (int64, error) {
	id, err := strconv.ParseInt(indexerID, 36, 64)
	if err != nil {
		return 0, fmt.Errorf("Unexpected indexer ID %s: %v", indexerID, err)
	}
	return id, nil
}

// numericEqualityQuery a numeric equality query for the given value and field
func numericEqualityQuery(value int64, field string) *query.NumericRangeQuery {
	f := float64(value)
	tru := true
	q := bleve.NewNumericRangeInclusiveQuery(&f, &f, &tru, &tru)
	q.SetField(field)
	return q
}

func newMatchPhraseQuery(matchPhrase, field, analyzer string) *query.MatchPhraseQuery {
	q := bleve.NewMatchPhraseQuery(matchPhrase)
	q.FieldVal = field
	q.Analyzer = analyzer
	return q
}

const unicodeNormalizeName = "unicodeNormalize"

func addUnicodeNormalizeTokenFilter(m *mapping.IndexMappingImpl) error {
	return m.AddCustomTokenFilter(unicodeNormalizeName, map[string]interface{}{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	})
}

// createIndexer create a repo indexer if one does not already exist
func createIndexer(path string, latestVersion int) (bleve.Index, error) {
	var err error
	docMapping := bleve.NewDocumentMapping()
	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)

	mapping := bleve.NewIndexMapping()
	if err = addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return nil, err
	} else if err = mapping.AddCustomAnalyzer(repoIndexerAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, camelcase.Name, lowercase.Name, unique.Name},
	}); err != nil {
		return nil, err
	}
	mapping.DefaultAnalyzer = repoIndexerAnalyzer
	mapping.AddDocumentMapping(repoIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())

	repoIndexer, err := bleve.New(path, mapping)
	if err != nil {
		return nil, err
	}
	return repoIndexer, rupture.WriteIndexMetadata(path, &rupture.IndexMetadata{
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

// openIndexer open the index at the specified path, checking for metadata
// updates and bleve version updates.  If index needs to be created (or
// re-created), returns (nil, nil)
func openIndexer(path string, latestVersion int) (bleve.Index, error) {
	_, err := os.Stat(setting.Indexer.IssuePath)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	metadata, err := rupture.ReadIndexMetadata(path)
	if err != nil {
		return nil, err
	}
	if metadata.Version < latestVersion {
		// the indexer is using a previous version, so we should delete it and
		// re-populate
		return nil, os.RemoveAll(path)
	}

	index, err := bleve.Open(path)
	if err != nil && err == upsidedown.IncompatibleVersion {
		// the indexer was built with a previous version of bleve, so we should
		// delete it and re-populate
		return nil, os.RemoveAll(path)
	} else if err != nil {
		return nil, err
	}
	return index, nil
}

const (
	maxBatchSize             = 16
	repoIndexerAnalyzer      = "repoIndexerAnalyzer"
	repoIndexerDocType       = "repoIndexerDocType"
	repoIndexerLatestVersion = 1
)

var (
	_ Indexer = &BleveIndexer{}
)

// BleveIndexer represents a bleve indexer implementation
type BleveIndexer struct {
	indexDir string
	indexer  bleve.Index // indexer (thread-safe) index for repository contents
}

// NewBleveIndexer creates a new bleve local indexer
func NewBleveIndexer(indexDir string) *BleveIndexer {
	return &BleveIndexer{
		indexDir: indexDir,
	}
}

// Init init the indexer
func (b *BleveIndexer) Init() (bool, error) {
	var err error
	b.indexer, err = openIndexer(b.indexDir, repoIndexerLatestVersion)
	if err != nil {
		return false, err
	}
	if b.indexer != nil {
		return true, nil
	}

	b.indexer, err = createIndexer(b.indexDir, repoIndexerLatestVersion)
	return false, err
}

// Index indexes the data
func (b *BleveIndexer) Index(datas []*IndexerData) error {
	for _, data := range datas {
		repo, err := models.GetRepositoryByID(data.RepoID)
		if err != nil {
			return err
		}

		sha, err := getDefaultBranchSha(repo)
		if err != nil {
			return err
		}
		changes, err := getRepoChanges(repo, sha)
		if err != nil {
			return err
		} else if changes == nil {
			return nil
		}

		batch := rupture.NewFlushingBatch(b.indexer, maxBatchSize)
		for _, update := range changes.Updates {
			if err := addUpdate(update, repo, batch); err != nil {
				return err
			}
		}
		for _, filename := range changes.RemovedFilenames {
			if err := batch.Delete(filenameIndexerID(repo.ID, filename)); err != nil {
				return err
			}
		}
		if err = batch.Flush(); err != nil {
			return err
		}

		if err := repo.UpdateIndexerStatus(sha); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes indexes by ids
func (b *BleveIndexer) Delete(repoIDs ...int64) error {
	if len(repoIDs) <= 0 {
		return errors.New("no repo id given")
	}

	var repoQueries = make([]query.Query, 0, len(repoIDs))
	for _, repoID := range repoIDs {
		repoQueries = append(repoQueries, numericEqualityQuery(repoID, "RepoID"))
	}

	query := bleve.NewConjunctionQuery(
		bleve.NewDisjunctionQuery(repoQueries...),
	)

	searchRequest := bleve.NewSearchRequestOptions(query, 2147483647, 0, false)
	result, err := b.indexer.Search(searchRequest)
	if err != nil {
		return err
	}
	batch := rupture.NewFlushingBatch(b.indexer, maxBatchSize)
	for _, hit := range result.Hits {
		if err = batch.Delete(hit.ID); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Search searches for files in the specified repo.
// Returns the matching file-paths
func (b *BleveIndexer) Search(repoIDs []int64, keyword string, page, pageSize int) (*SearchResult, error) {
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

	result, err := b.indexer.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	matches := make([]Match, len(result.Hits))
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
		matches[i] = Match{
			RepoID:     int64(hit.Fields["RepoID"].(float64)),
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Filename:   filenameOfIndexerID(hit.ID),
			Content:    hit.Fields["Content"].(string),
		}
	}
	return &SearchResult{
		Total: result.Total,
		Hits:  matches,
	}, nil
}

// RepoIndexerData data stored in the repo indexer
type RepoIndexerData IndexerData

// Type returns the document type, for bleve's mapping.Classifier interface.
func (d *RepoIndexerData) Type() string {
	return repoIndexerDocType
}
