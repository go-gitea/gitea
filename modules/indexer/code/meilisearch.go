// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/typesniffer"

	"github.com/meilisearch/meilisearch-go"
)

var _ Indexer = &MeilisearchIndexer{}

// MeilisearchIndexer implements Indexer interface
type MeilisearchIndexer struct {
	client               *meilisearch.Client
	indexerName          string
	available            bool
	availabilityCallback func(bool)
	stopTimer            chan struct{}
	lock                 sync.RWMutex
}

// NewMeilisearchIndexer creates a new meilisearch indexer
func NewMeilisearchIndexer(url, apiKey, indexerName string) (*MeilisearchIndexer, bool, error) {
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   url,
		APIKey: apiKey,
	})

	indexer := &MeilisearchIndexer{
		client:      client,
		indexerName: indexerName,
		available:   true,
		stopTimer:   make(chan struct{}),
	}

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				indexer.checkAvailability()
			case <-indexer.stopTimer:
				ticker.Stop()
				return
			}
		}
	}()

	_, err := indexer.client.GetIndex(indexer.indexerName)
	if err == nil {
		// if no error that means the index already exists
		return indexer, true, nil
	}

	_, err = indexer.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        indexerName,
		PrimaryKey: "id",
	})
	if err != nil {
		return indexer, false, err
	}

	_, err = indexer.client.Index(indexerName).UpdateFilterableAttributes(&[]string{"repo_id"})

	return indexer, false, err
}

// Init will initialize the indexer
func (b *MeilisearchIndexer) init() (bool, error) {
	_, err := b.client.GetIndex(b.indexerName)
	if err == nil {
		return true, nil
	}
	_, err = b.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        b.indexerName,
		PrimaryKey: "id",
	})
	if err != nil {
		return false, b.checkError(err)
	}

	_, err = b.client.Index(b.indexerName).UpdateFilterableAttributes(&[]string{"repo_id"})
	return false, b.checkError(err)
}

// SetAvailabilityChangeCallback sets callback that will be triggered when availability changes
func (b *MeilisearchIndexer) SetAvailabilityChangeCallback(callback func(bool)) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.availabilityCallback = callback
}

// Ping checks if meili is available
func (b *MeilisearchIndexer) Ping() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.available
}

func (b *MeilisearchIndexer) addUpdate(ctx context.Context, batchWriter git.WriteCloserError, batchReader *bufio.Reader, sha string, update fileUpdate, repo *repo_model.Repository) (meiliItem, error) {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return meiliItem{}, nil
	}

	size := update.Size
	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = git.NewCommand(ctx, "cat-file", "-s").AddDynamicArguments(update.BlobSha).RunStdString(&git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			return meiliItem{}, err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return meiliItem{}, fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}

	id := filenameIndexerID(repo.ID, update.Filename)

	if size > setting.Indexer.MaxIndexerFileSize {
		// file too big, delete it
		return meiliItem{
			ID:     id,
			Action: mActionDelete,
		}, nil
	}

	if _, err := batchWriter.Write([]byte(update.BlobSha + "\n")); err != nil {
		return meiliItem{}, err
	}

	_, _, size, err = git.ReadBatchLine(batchReader)
	if err != nil {
		return meiliItem{}, err
	}

	fileContents, err := io.ReadAll(io.LimitReader(batchReader, size))
	if err != nil {
		return meiliItem{}, err
	} else if !typesniffer.DetectContentType(fileContents).IsText() {
		// FIXME: UTF-16 files will probably fail here
		return meiliItem{}, nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return meiliItem{}, err
	}

	return meiliItem{
		ID:     id,
		Action: mActionCreate,
		Doc: map[string]interface{}{
			"id":         id,
			"repo_id":    repo.ID,
			"content":    string(charset.ToUTF8DropErrors(fileContents)),
			"commit_id":  sha,
			"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
			"updated_at": timeutil.TimeStampNow(),
		},
	}, nil
}

type mAction int

const (
	mActionCreate mAction = iota
	mActionDelete
)

type meiliItem struct {
	Action mAction
	ID     string
	Doc    map[string]interface{}
}

// Index will save the index data
func (b *MeilisearchIndexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *repoChanges) error {
	reqs := make([]meiliItem, 0)
	if len(changes.Updates) > 0 {
		// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
		if err := git.EnsureValidGitRepository(ctx, repo.RepoPath()); err != nil {
			log.Error("Unable to open git repo: %s for %-v: %v", repo.RepoPath(), repo, err)
			return err
		}

		batchWriter, batchReader, cancel := git.CatFileBatch(ctx, repo.RepoPath())
		defer cancel()

		for _, update := range changes.Updates {
			updateReq, err := b.addUpdate(ctx, batchWriter, batchReader, sha, update, repo)
			if err != nil {
				return err
			}
			if updateReq.ID != "" {
				reqs = append(reqs, updateReq)
			}
		}
		cancel()
	}

	for _, filename := range changes.RemovedFilenames {
		reqs = append(reqs, meiliItem{
			ID:     filenameIndexerID(repo.ID, filename),
			Action: mActionDelete,
		})
	}

	for _, req := range reqs {
		switch req.Action {
		case mActionCreate:
			_, err := b.client.Index(b.indexerName).AddDocuments(req.Doc)
			if err != nil {
				return b.checkError(err)
			}
		case mActionDelete:
			_, err := b.client.Index(b.indexerName).DeleteDocument(req.ID)
			if err != nil {
				return b.checkError(err)
			}
		}
	}
	return nil
}

// Delete deletes indexes by ids
func (b *MeilisearchIndexer) Delete(repoID int64) error {
	// TODO: Delete all documents by repo_id
	return nil
}

func convertMeiliResult(searchResult *meilisearch.SearchResponse, kw string, pageSize int) (int64, []*SearchResult, []*SearchResultLanguages, error) {
	// TODO: convert search response into SearchResult
	return 0, []*SearchResult{}, []*SearchResultLanguages{}, nil
}

func extractMeiliAggs(searchResult *meilisearch.SearchResponse) []*SearchResultLanguages {
	// TODO: extract search languages into aggregates
	return []*SearchResultLanguages{}
}

// Search searches for codes and language stats by given conditions.
func (b *MeilisearchIndexer) Search(ctx context.Context, repoIDs []int64, language, keyword string, page, pageSize int, isMatch bool) (int64, []*SearchResult, []*SearchResultLanguages, error) {
	// TODO: search
	return 0, []*SearchResult{}, []*SearchResultLanguages{}, nil

}

// Close implements indexer
func (b *MeilisearchIndexer) Close() {
	select {
	case <-b.stopTimer:
	default:
		close(b.stopTimer)
	}
}

func (b *MeilisearchIndexer) checkError(err error) error {
	return err
}

func (b *MeilisearchIndexer) checkAvailability() {
	_, err := b.client.Health()
	if err != nil {
		b.setAvailability(false)
		return
	}
	b.setAvailability(true)
}

func (b *MeilisearchIndexer) setAvailability(available bool) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.available == available {
		return
	}

	b.available = available
	if b.availabilityCallback != nil {
		// Call the callback from within the lock to ensure that the ordering remains correct
		b.availabilityCallback(b.available)
	}
}
