// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/ethantkoenig/rupture"
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
	repoIndexerOperationQueue = make(chan repoIndexerOperation, setting.Indexer.UpdateQueueLength)
	indexer.InitRepoIndexer(populateRepoIndexerAsynchronously)
	go processRepoIndexerOperationQueue()
}

// populateRepoIndexerAsynchronously asynchronously populates the repo indexer
// with pre-existing data. This should only be run when the indexer is created
// for the first time.
func populateRepoIndexerAsynchronously() error {
	exist, err := x.Table("repository").Exist()
	if err != nil {
		return err
	} else if !exist {
		return nil
	}

	// if there is any existing repo indexer metadata in the DB, delete it
	// since we are starting afresh. Also, xorm requires deletes to have a
	// condition, and we want to delete everything, thus 1=1.
	if _, err := x.Where("1=1").Delete(new(RepoIndexerStatus)); err != nil {
		return err
	}

	var maxRepoID int64
	if _, err = x.Select("MAX(id)").Table("repository").Get(&maxRepoID); err != nil {
		return err
	}
	go populateRepoIndexer(maxRepoID)
	return nil
}

// populateRepoIndexer populate the repo indexer with pre-existing data. This
// should only be run when the indexer is created for the first time.
func populateRepoIndexer(maxRepoID int64) {
	log.Info("Populating the repo indexer with existing repositories")
	// start with the maximum existing repo ID and work backwards, so that we
	// don't include repos that are created after gitea starts; such repos will
	// already be added to the indexer, and we don't need to add them again.
	for maxRepoID > 0 {
		repos := make([]*Repository, 0, RepositoryListDefaultPageSize)
		err := x.Where("id <= ?", maxRepoID).
			OrderBy("id DESC").
			Limit(RepositoryListDefaultPageSize).
			Find(&repos)
		if err != nil {
			log.Error(4, "populateRepoIndexer: %v", err)
			return
		} else if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			repoIndexerOperationQueue <- repoIndexerOperation{
				repo:    repo,
				deleted: false,
			}
			maxRepoID = repo.ID - 1
		}
	}
	log.Info("Done populating the repo indexer with existing repositories")
}

func updateRepoIndexer(repo *Repository) error {
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

	batch := indexer.RepoIndexerBatch()
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
	return repo.updateIndexerStatus(sha)
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

func getDefaultBranchSha(repo *Repository) (string, error) {
	stdout, err := git.NewCommand("show-ref", "-s", repo.DefaultBranch).RunInDir(repo.RepoPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// getRepoChanges returns changes to repo since last indexer update
func getRepoChanges(repo *Repository, revision string) (*repoChanges, error) {
	if err := repo.getIndexerStatus(); err != nil {
		return nil, err
	}

	if len(repo.IndexerStatus.CommitSha) == 0 {
		return genesisChanges(repo, revision)
	}
	return nonGenesisChanges(repo, revision)
}

func addUpdate(update fileUpdate, repo *Repository, batch rupture.FlushingBatch) error {
	stdout, err := git.NewCommand("cat-file", "-s", update.BlobSha).
		RunInDir(repo.RepoPath())
	if err != nil {
		return err
	}
	if size, err := strconv.Atoi(strings.TrimSpace(stdout)); err != nil {
		return fmt.Errorf("Misformatted git cat-file output: %v", err)
	} else if int64(size) > setting.Indexer.MaxIndexerFileSize {
		return nil
	}

	fileContents, err := git.NewCommand("cat-file", "blob", update.BlobSha).
		RunInDirBytes(repo.RepoPath())
	if err != nil {
		return err
	} else if !base.IsTextFile(fileContents) {
		return nil
	}
	indexerUpdate := indexer.RepoIndexerUpdate{
		Filepath: update.Filename,
		Op:       indexer.RepoIndexerOpUpdate,
		Data: &indexer.RepoIndexerData{
			RepoID:  repo.ID,
			Content: string(fileContents),
		},
	}
	return indexerUpdate.AddToFlushingBatch(batch)
}

func addDelete(filename string, repo *Repository, batch rupture.FlushingBatch) error {
	indexerUpdate := indexer.RepoIndexerUpdate{
		Filepath: filename,
		Op:       indexer.RepoIndexerOpDelete,
		Data: &indexer.RepoIndexerData{
			RepoID: repo.ID,
		},
	}
	return indexerUpdate.AddToFlushingBatch(batch)
}

// parseGitLsTreeOutput parses the output of a `git ls-tree -r --full-name` command
func parseGitLsTreeOutput(stdout []byte) ([]fileUpdate, error) {
	entries, err := git.ParseTreeEntries(stdout)
	if err != nil {
		return nil, err
	}
	updates := make([]fileUpdate, len(entries))
	for i, entry := range entries {
		updates[i] = fileUpdate{
			Filename: entry.Name(),
			BlobSha:  entry.ID.String(),
		}
	}
	return updates, nil
}

// genesisChanges get changes to add repo to the indexer for the first time
func genesisChanges(repo *Repository, revision string) (*repoChanges, error) {
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
func nonGenesisChanges(repo *Repository, revision string) (*repoChanges, error) {
	diffCmd := git.NewCommand("diff", "--name-status",
		repo.IndexerStatus.CommitSha, revision)
	stdout, err := diffCmd.RunInDir(repo.RepoPath())
	if err != nil {
		// previous commit sha may have been removed by a force push, so
		// try rebuilding from scratch
		log.Warn("git diff: %v", err)
		if err = indexer.DeleteRepoFromIndexer(repo.ID); err != nil {
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
