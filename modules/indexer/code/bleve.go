// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/ethantkoenig/rupture"
)

type repoIndexerOperation struct {
	repoID   int64
	deleted  bool
	watchers []chan<- error
}

var repoIndexerOperationQueue chan repoIndexerOperation

// InitRepoIndexer initialize the repo indexer
func InitRepoIndexer() {
	if !setting.Indexer.RepoIndexerEnabled {
		return
	}
	waitChannel := make(chan time.Duration)
	repoIndexerOperationQueue = make(chan repoIndexerOperation, setting.Indexer.UpdateQueueLength)
	go func() {
		start := time.Now()
		log.Info("Initializing Repository Indexer")
		initRepoIndexer(populateRepoIndexerAsynchronously)
		go processRepoIndexerOperationQueue()
		waitChannel <- time.Since(start)
	}()
	if setting.Indexer.StartupTimeout > 0 {
		go func() {
			timeout := setting.Indexer.StartupTimeout
			if graceful.Manager.IsChild() && setting.GracefulHammerTime > 0 {
				timeout += setting.GracefulHammerTime
			}
			select {
			case duration := <-waitChannel:
				log.Info("Repository Indexer Initialization took %v", duration)
			case <-time.After(timeout):
				log.Fatal("Repository Indexer Initialization Timed-Out after: %v", timeout)
			}
		}()

	}
}

// populateRepoIndexerAsynchronously asynchronously populates the repo indexer
// with pre-existing data. This should only be run when the indexer is created
// for the first time.
func populateRepoIndexerAsynchronously() error {
	exist, err := models.IsTableNotEmpty("repository")
	if err != nil {
		return err
	} else if !exist {
		return nil
	}

	// if there is any existing repo indexer metadata in the DB, delete it
	// since we are starting afresh. Also, xorm requires deletes to have a
	// condition, and we want to delete everything, thus 1=1.
	if err := models.DeleteAllRecords("repo_indexer_status"); err != nil {
		return err
	}

	var maxRepoID int64
	if maxRepoID, err = models.GetMaxID("repository"); err != nil {
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
		repos := make([]*models.Repository, 0, models.RepositoryListDefaultPageSize)
		err := models.FindByMaxID(maxRepoID, models.RepositoryListDefaultPageSize, &repos)
		if err != nil {
			log.Error("populateRepoIndexer: %v", err)
			return
		} else if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			repoIndexerOperationQueue <- repoIndexerOperation{
				repoID:  repo.ID,
				deleted: false,
			}
			maxRepoID = repo.ID - 1
		}
	}
	log.Info("Done populating the repo indexer with existing repositories")
}

func updateRepoIndexer(repoID int64) error {
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

	batch := RepoIndexerBatch()
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
	stdout, err := git.NewCommand("show-ref", "-s", repo.DefaultBranch).RunInDir(repo.RepoPath())
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
		if err = deleteRepoFromIndexer(repo.ID); err != nil {
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
		var err error
		if op.deleted {
			if err = deleteRepoFromIndexer(op.repoID); err != nil {
				log.Error("deleteRepoFromIndexer: %v", err)
			}
		} else {
			if err = updateRepoIndexer(op.repoID); err != nil {
				log.Error("updateRepoIndexer: %v", err)
			}
		}
		for _, watcher := range op.watchers {
			watcher <- err
		}
	}
}

// DeleteRepoFromIndexer remove all of a repository's entries from the indexer
func DeleteRepoFromIndexer(repo *models.Repository, watchers ...chan<- error) {
	addOperationToQueue(repoIndexerOperation{repoID: repo.ID, deleted: true, watchers: watchers})
}

// UpdateRepoIndexer update a repository's entries in the indexer
func UpdateRepoIndexer(repo *models.Repository, watchers ...chan<- error) {
	addOperationToQueue(repoIndexerOperation{repoID: repo.ID, deleted: false, watchers: watchers})
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
