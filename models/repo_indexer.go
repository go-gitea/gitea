// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
)

// RepoIndexerStatus status of a repo's entry in the repo indexer
// For now, implicitly refers to default branch
type RepoIndexerStatus struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX"`
	CommitSha string `xorm:"VARCHAR(40)"`
}

func (repo *Repository) getIndexerStatus() error {
	if repo.IndexerStatus != nil {
		return nil
	}
	status := &RepoIndexerStatus{RepoID: repo.ID}
	has, err := x.Get(status)
	if err != nil {
		return err
	} else if !has {
		status.CommitSha = ""
	}
	repo.IndexerStatus = status
	return nil
}

func (repo *Repository) updateIndexerStatus(sha string) error {
	if err := repo.getIndexerStatus(); err != nil {
		return err
	}
	if len(repo.IndexerStatus.CommitSha) == 0 {
		repo.IndexerStatus.CommitSha = sha
		_, err := x.Insert(repo.IndexerStatus)
		return err
	}
	repo.IndexerStatus.CommitSha = sha
	_, err := x.ID(repo.IndexerStatus.ID).Cols("commit_sha").
		Update(repo.IndexerStatus)
	return err
}

type repoIndexerOperation struct {
	repo    *Repository
	deleted bool
}

var repoIndexerOperationQueue chan repoIndexerOperation

// InitRepoIndexer initialize the repo indexer
func InitRepoIndexer() {
	if !setting.Indexer.RepoIndexerEnabled {
		return
	}
	indexer.InitRepoIndexer(populateRepoIndexer)
	repoIndexerOperationQueue = make(chan repoIndexerOperation, setting.Indexer.UpdateQueueLength)
	go processRepoIndexerOperationQueue()
}

// populateRepoIndexer populate the repo indexer with data
func populateRepoIndexer() error {
	log.Info("Populating repository indexer (this may take a while)")
	for page := 1; ; page++ {
		repos, _, err := SearchRepositoryByName(&SearchRepoOptions{
			Page:     page,
			PageSize: 10,
			OrderBy:  SearchOrderByID,
			Private:  true,
		})
		if err != nil {
			return err
		} else if len(repos) == 0 {
			return nil
		}
		for _, repo := range repos {
			if err = updateRepoIndexer(repo); err != nil {
				// only log error, since this should not prevent
				// gitea from starting up
				log.Error(4, "updateRepoIndexer: repoID=%d, %v", repo.ID, err)
			}
		}
	}
}

func updateRepoIndexer(repo *Repository) error {
	changes, err := getRepoChanges(repo)
	if err != nil {
		return err
	} else if changes == nil {
		return nil
	}

	batch := indexer.RepoIndexerBatch()
	for _, filename := range changes.UpdatedFiles {
		if err := addUpdate(filename, repo, batch); err != nil {
			return err
		}
	}
	for _, filename := range changes.RemovedFiles {
		if err := addDelete(filename, repo, batch); err != nil {
			return err
		}
	}
	if err = batch.Flush(); err != nil {
		return err
	}
	return updateLastIndexSync(repo)
}

// repoChanges changes (file additions/updates/removals) to a repo
type repoChanges struct {
	UpdatedFiles []string
	RemovedFiles []string
}

// getRepoChanges returns changes to repo since last indexer update
func getRepoChanges(repo *Repository) (*repoChanges, error) {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err := repo.UpdateLocalCopyBranch(""); err != nil {
		return nil, err
	} else if !git.IsBranchExist(repo.LocalCopyPath(), repo.DefaultBranch) {
		// repo does not have any commits yet, so nothing to update
		return nil, nil
	} else if err = repo.UpdateLocalCopyBranch(repo.DefaultBranch); err != nil {
		return nil, err
	} else if err = repo.getIndexerStatus(); err != nil {
		return nil, err
	}

	if len(repo.IndexerStatus.CommitSha) == 0 {
		return genesisChanges(repo)
	}
	return nonGenesisChanges(repo)
}

func addUpdate(filename string, repo *Repository, batch *indexer.Batch) error {
	filepath := path.Join(repo.LocalCopyPath(), filename)
	if stat, err := os.Stat(filepath); err != nil {
		return err
	} else if stat.Size() > setting.Indexer.MaxIndexerFileSize {
		return nil
	} else if stat.IsDir() {
		// file could actually be a directory, if it is the root of a submodule.
		// We do not index submodule contents, so don't do anything.
		return nil
	}
	fileContents, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	} else if !base.IsTextFile(fileContents) {
		return nil
	}
	return batch.Add(indexer.RepoIndexerUpdate{
		Filepath: filename,
		Op:       indexer.RepoIndexerOpUpdate,
		Data: &indexer.RepoIndexerData{
			RepoID:  repo.ID,
			Content: string(fileContents),
		},
	})
}

func addDelete(filename string, repo *Repository, batch *indexer.Batch) error {
	return batch.Add(indexer.RepoIndexerUpdate{
		Filepath: filename,
		Op:       indexer.RepoIndexerOpDelete,
		Data: &indexer.RepoIndexerData{
			RepoID: repo.ID,
		},
	})
}

// genesisChanges get changes to add repo to the indexer for the first time
func genesisChanges(repo *Repository) (*repoChanges, error) {
	var changes repoChanges
	stdout, err := git.NewCommand("ls-files").RunInDir(repo.LocalCopyPath())
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(stdout, "\n") {
		filename := strings.TrimSpace(line)
		if len(filename) == 0 {
			continue
		} else if filename[0] == '"' {
			filename, err = strconv.Unquote(filename)
			if err != nil {
				return nil, err
			}
		}
		changes.UpdatedFiles = append(changes.UpdatedFiles, filename)
	}
	return &changes, nil
}

// nonGenesisChanges get changes since the previous indexer update
func nonGenesisChanges(repo *Repository) (*repoChanges, error) {
	diffCmd := git.NewCommand("diff", "--name-status",
		repo.IndexerStatus.CommitSha, "HEAD")
	stdout, err := diffCmd.RunInDir(repo.LocalCopyPath())
	if err != nil {
		// previous commit sha may have been removed by a force push, so
		// try rebuilding from scratch
		if err = indexer.DeleteRepoFromIndexer(repo.ID); err != nil {
			return nil, err
		}
		return genesisChanges(repo)
	}
	var changes repoChanges
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
			changes.UpdatedFiles = append(changes.UpdatedFiles, filename)
		case 'D':
			changes.RemovedFiles = append(changes.RemovedFiles, filename)
		default:
			log.Warn("Unrecognized status: %c (line=%s)", status, line)
		}
	}
	return &changes, nil
}

func updateLastIndexSync(repo *Repository) error {
	stdout, err := git.NewCommand("rev-parse", "HEAD").RunInDir(repo.LocalCopyPath())
	if err != nil {
		return err
	}
	sha := strings.TrimSpace(stdout)
	return repo.updateIndexerStatus(sha)
}

func processRepoIndexerOperationQueue() {
	for {
		op := <-repoIndexerOperationQueue
		if op.deleted {
			if err := indexer.DeleteRepoFromIndexer(op.repo.ID); err != nil {
				log.Error(4, "DeleteRepoFromIndexer: %v", err)
			}
		} else {
			if err := updateRepoIndexer(op.repo); err != nil {
				log.Error(4, "updateRepoIndexer: %v", err)
			}
		}
	}
}

// DeleteRepoFromIndexer remove all of a repository's entries from the indexer
func DeleteRepoFromIndexer(repo *Repository) {
	addOperationToQueue(repoIndexerOperation{repo: repo, deleted: true})
}

// UpdateRepoIndexer update a repository's entries in the indexer
func UpdateRepoIndexer(repo *Repository) {
	addOperationToQueue(repoIndexerOperation{repo: repo, deleted: false})
}

func addOperationToQueue(op repoIndexerOperation) {
	if !setting.Indexer.RepoIndexerEnabled {
		return
	}
	select {
	case repoIndexerOperationQueue <- op:
		break
	default:
		go func() {
			repoIndexerOperationQueue <- op
		}()
	}
}
