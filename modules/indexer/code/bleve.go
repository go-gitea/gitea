// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	"github.com/blevesearch/bleve/analysis/token/unicodenorm"
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

// numericEqualityQuery a numeric equality query for the given value and field
func numericEqualityQuery(value int64, field string) *query.NumericRangeQuery {
	f := float64(value)
	tru := true
	q := bleve.NewNumericRangeInclusiveQuery(&f, &f, &tru, &tru)
	q.SetField(field)
	return q
}

const unicodeNormalizeName = "unicodeNormalize"

func addUnicodeNormalizeTokenFilter(m *mapping.IndexMappingImpl) error {
	return m.AddCustomTokenFilter(unicodeNormalizeName, map[string]interface{}{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	})
}

const maxBatchSize = 16

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

// repoChanges changes (file additions/updates/removals) to a repo
type repoChanges struct {
	Updates          []fileUpdate
	RemovedFilenames []string
}

type fileUpdate struct {
	Filename string
	BlobSha  string
}

func getDefaultBranchSha(repo *models.Repository) (string, error) {
	stdout, err := git.NewCommand("show-ref", "-s", git.BranchPrefix+repo.DefaultBranch).RunInDir(repo.RepoPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// getRepoChanges returns changes to repo since last indexer update
func getRepoChanges(repo *models.Repository, revision string) (*repoChanges, error) {
	if err := repo.GetIndexerStatus(); err != nil {
		return nil, err
	}

	if len(repo.IndexerStatus.CommitSha) == 0 {
		return genesisChanges(repo, revision)
	}
	return nonGenesisChanges(repo, revision)
}

func addUpdate(update fileUpdate, repo *models.Repository, batch rupture.FlushingBatch) error {
	stdout, err := git.NewCommand("cat-file", "-s", update.BlobSha).
		RunInDir(repo.RepoPath())
	if err != nil {
		return err
	}
	if size, err := strconv.Atoi(strings.TrimSpace(stdout)); err != nil {
		return fmt.Errorf("Misformatted git cat-file output: %v", err)
	} else if int64(size) > setting.Indexer.MaxIndexerFileSize {
		return addDelete(update.Filename, repo, batch)
	}

	fileContents, err := git.NewCommand("cat-file", "blob", update.BlobSha).
		RunInDirBytes(repo.RepoPath())
	if err != nil {
		return err
	} else if !base.IsTextFile(fileContents) {
		// FIXME: UTF-16 files will probably fail here
		return nil
	}
	indexerUpdate := RepoIndexerUpdate{
		Filepath: update.Filename,
		Op:       RepoIndexerOpUpdate,
		Data: &RepoIndexerData{
			RepoID:  repo.ID,
			Content: string(charset.ToUTF8DropErrors(fileContents)),
		},
	}
	return indexerUpdate.AddToFlushingBatch(batch)
}

func addDelete(filename string, repo *models.Repository, batch rupture.FlushingBatch) error {
	indexerUpdate := RepoIndexerUpdate{
		Filepath: filename,
		Op:       RepoIndexerOpDelete,
		Data: &RepoIndexerData{
			RepoID: repo.ID,
		},
	}
	return indexerUpdate.AddToFlushingBatch(batch)
}

func isIndexable(entry *git.TreeEntry) bool {
	if !entry.IsRegular() && !entry.IsExecutable() {
		return false
	}
	name := strings.ToLower(entry.Name())
	for _, g := range setting.Indexer.ExcludePatterns {
		if g.Match(name) {
			return false
		}
	}
	for _, g := range setting.Indexer.IncludePatterns {
		if g.Match(name) {
			return true
		}
	}
	return len(setting.Indexer.IncludePatterns) == 0
}

// parseGitLsTreeOutput parses the output of a `git ls-tree -r --full-name` command
func parseGitLsTreeOutput(stdout []byte) ([]fileUpdate, error) {
	entries, err := git.ParseTreeEntries(stdout)
	if err != nil {
		return nil, err
	}
	var idxCount = 0
	updates := make([]fileUpdate, len(entries))
	for _, entry := range entries {
		if isIndexable(entry) {
			updates[idxCount] = fileUpdate{
				Filename: entry.Name(),
				BlobSha:  entry.ID.String(),
			}
			idxCount++
		}
	}
	return updates[:idxCount], nil
}

// genesisChanges get changes to add repo to the indexer for the first time
func genesisChanges(repo *models.Repository, revision string) (*repoChanges, error) {
	var changes repoChanges
	stdout, err := git.NewCommand("ls-tree", "--full-tree", "-r", revision).
		RunInDirBytes(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	changes.Updates, err = parseGitLsTreeOutput(stdout)
	return &changes, err
}

// nonGenesisChanges get changes since the previous indexer update
func nonGenesisChanges(repo *models.Repository, revision string) (*repoChanges, error) {
	diffCmd := git.NewCommand("diff", "--name-status",
		repo.IndexerStatus.CommitSha, revision)
	stdout, err := diffCmd.RunInDir(repo.RepoPath())
	if err != nil {
		// previous commit sha may have been removed by a force push, so
		// try rebuilding from scratch
		log.Warn("git diff: %v", err)
		if err = indexer.Delete(repo.ID); err != nil {
			return nil, err
		}
		return genesisChanges(repo, revision)
	}
	var changes repoChanges
	updatedFilenames := make([]string, 0, 10)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		filename := strings.TrimSpace(line[1:])
		if len(filename) == 0 {
			continue
		} else if filename[0] == '"' {
			filename, err = strconv.Unquote(filename)
			if err != nil {
				return nil, err
			}
		}

		switch status := line[0]; status {
		case 'M', 'A':
			updatedFilenames = append(updatedFilenames, filename)
		case 'D':
			changes.RemovedFilenames = append(changes.RemovedFilenames, filename)
		default:
			log.Warn("Unrecognized status: %c (line=%s)", status, line)
		}
	}

	cmd := git.NewCommand("ls-tree", "--full-tree", revision, "--")
	cmd.AddArguments(updatedFilenames...)
	lsTreeStdout, err := cmd.RunInDirBytes(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	changes.Updates, err = parseGitLsTreeOutput(lsTreeStdout)
	return &changes, err
}

const (
	repoIndexerAnalyzer      = "repoIndexerAnalyzer"
	repoIndexerDocType       = "repoIndexerDocType"
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

// createRepoIndexer create a repo indexer if one does not already exist
func createRepoIndexer(path string, latestVersion int) (bleve.Index, error) {
	docMapping := bleve.NewDocumentMapping()
	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)

	mapping := bleve.NewIndexMapping()
	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return nil, err
	} else if err := mapping.AddCustomAnalyzer(repoIndexerAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, lowercase.Name},
	}); err != nil {
		return nil, err
	}
	mapping.DefaultAnalyzer = repoIndexerAnalyzer
	mapping.AddDocumentMapping(repoIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())

	indexer, err := bleve.New(path, mapping)
	if err != nil {
		return nil, err
	}

	if err = rupture.WriteIndexMetadata(path, &rupture.IndexMetadata{
		Version: latestVersion,
	}); err != nil {
		return nil, err
	}
	return indexer, nil
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

var (
	_ Indexer = &BleveIndexer{}
)

// BleveIndexer represents a bleve indexer implementation
type BleveIndexer struct {
	indexDir      string
	indexerHolder *bleveIndexerHolder
}

// NewBleveIndexer creates a new bleve local indexer
func NewBleveIndexer(indexDir string) *BleveIndexer {
	return &BleveIndexer{
		indexDir:      indexDir,
		indexerHolder: newBleveIndexerHolder(),
	}
}

// Init init the indexer
func (b *BleveIndexer) Init() (bool, error) {
	indexer, err := openIndexer(b.indexDir, repoIndexerLatestVersion)
	if err != nil {
		log.Fatal("openIndexer: %v", err)
	}
	if indexer != nil {
		b.indexerHolder.set(indexer)
		b.closeAtTerminate()
		return false, nil
	}

	indexer, err = createRepoIndexer(setting.Indexer.RepoPath, repoIndexerLatestVersion)
	if err != nil {
		return false, err
	}
	b.indexerHolder.set(indexer)
	b.closeAtTerminate()
	return true, nil
}

func (b *BleveIndexer) closeAtTerminate() {
	graceful.GetManager().RunAtTerminate(context.Background(), func() {
		log.Debug("Closing repo indexer")
		indexer := b.indexerHolder.get()
		if indexer != nil {
			err := indexer.Close()
			if err != nil {
				log.Error("Error whilst closing the repository indexer: %v", err)
			}
		}
		log.Info("PID: %d Repository Indexer closed", os.Getpid())
	})
}

// Index indexes the data
func (b *BleveIndexer) Index(repoID int64) error {
	repo, err := models.GetRepositoryByID(repoID)
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

	batch := rupture.NewFlushingBatch(b.indexerHolder.get(), maxBatchSize)
	for _, update := range changes.Updates {
		if err := addUpdate(update, repo, batch); err != nil {
			return err
		}
	}
	for _, filename := range changes.RemovedFilenames {
		if err := addDelete(filename, repo, batch); err != nil {
			return err
		}
	}
	if err = batch.Flush(); err != nil {
		return err
	}
	return repo.UpdateIndexerStatus(sha)
}

// Delete deletes indexes by ids
func (b *BleveIndexer) Delete(repoID int64) error {
	query := numericEqualityQuery(repoID, "RepoID")
	searchRequest := bleve.NewSearchRequestOptions(query, 2147483647, 0, false)
	result, err := b.indexerHolder.get().Search(searchRequest)
	if err != nil {
		return err
	}
	batch := rupture.NewFlushingBatch(b.indexerHolder.get(), maxBatchSize)
	for _, hit := range result.Hits {
		if err = batch.Delete(hit.ID); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Search searches for files in the specified repo.
// Returns the matching file-paths
func (b *BleveIndexer) Search(repoIDs []int64, keyword string, page, pageSize int) (int64, []*SearchResult, error) {
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

	result, err := b.indexerHolder.get().Search(searchRequest)
	if err != nil {
		return 0, nil, err
	}

	searchResults := make([]*SearchResult, len(result.Hits))
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
		searchResults[i] = &SearchResult{
			RepoID:     int64(hit.Fields["RepoID"].(float64)),
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Filename:   filenameOfIndexerID(hit.ID),
			Content:    hit.Fields["Content"].(string),
		}
	}
	return int64(result.Total), searchResults, nil
}
