// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package codes

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/ethantkoenig/rupture"
)

// IndexerData data stored in the issue indexer
type IndexerData struct {
	RepoID   int64
	Filepath string
	Content  string
	IsDelete bool
	RepoIDs  []int64
}

// Match represents on search result
type Match struct {
	RepoID     int64
	StartIndex int
	EndIndex   int
	Filename   string
	Content    string
}

// SearchResult represents search results
type SearchResult struct {
	Total uint64
	Hits  []Match
}

// type repoIndexerOperation struct {
// 	repo     *Repository
// 	deleted  bool
// 	watchers []chan<- error
// }

// Indexer defines an inteface to indexer issues contents
type Indexer interface {
	Init() (bool, error)
	Index(datas []*IndexerData) error
	Delete(repoIDs ...int64) error
	Search(repoIDs []int64, keyword string, page, pageSize int) (*SearchResult, error)
}

var (
	// codesIndexerQueue queue of issue ids to be updated
	codesIndexerQueue Queue
	codesIndexer      Indexer
)

// InitIndexer initialize codes indexer, syncReindex is true then reindex until
// all codes index done.
func InitIndexer(syncReindex bool) error {
	if !setting.Indexer.RepoIndexerEnabled {
		return nil
	}

	var populate bool
	switch setting.Indexer.RepoType {
	case "bleve":
		codesIndexer = NewBleveIndexer(setting.Indexer.RepoPath)
		exist, err := codesIndexer.Init()
		if err != nil {
			return err
		}
		populate = !exist
	default:
		return fmt.Errorf("unknow issue indexer type: %s", setting.Indexer.IssueType)
	}

	var err error
	switch setting.Indexer.CodesQueueType {
	case setting.LevelQueueType:
		codesIndexerQueue, err = NewLevelQueue(
			codesIndexer,
			setting.Indexer.CodesQueueDir,
			setting.Indexer.CodesQueueBatchNumber)
		if err != nil {
			return err
		}
	case setting.ChannelQueueType:
		codesIndexerQueue = NewChannelQueue(codesIndexer, setting.Indexer.CodesQueueBatchNumber)
	case setting.RedisQueueType:
		addrs, pass, idx, err := parseConnStr(setting.Indexer.CodesQueueConnStr)
		if err != nil {
			return err
		}
		codesIndexerQueue, err = NewRedisQueue(addrs, pass, idx, codesIndexer, setting.Indexer.CodesQueueBatchNumber)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Unsupported indexer queue type: %v", setting.Indexer.IssueQueueType)
	}

	go codesIndexerQueue.Run()

	if populate {
		if syncReindex {
			return populateRepoIndexer()
		}

		go func() {
			if err := populateRepoIndexer(); err != nil {
				log.Error("populateRepoIndexer: %v", err)
			}
		}()
	}

	return nil
}

// populateRepoIndexer populate the repo indexer with pre-existing data. This
// should only be run when the indexer is created for the first time.
func populateRepoIndexer() error {
	notEmpty, err := models.IsTableNotEmpty("repository")
	if err != nil {
		return err
	} else if !notEmpty {
		return nil
	}

	// if there is any existing repo indexer metadata in the DB, delete it
	// since we are starting afresh. Also, xorm requires deletes to have a
	// condition, and we want to delete everything, thus 1=1.
	if err := models.DeleteAllRecords("repo_indexer_status"); err != nil {
		return err
	}

	maxRepoID, err := models.GetMaxID("repository")
	if err != nil {
		return err
	}

	log.Info("Populating the repo indexer with existing repositories")
	// start with the maximum existing repo ID and work backwards, so that we
	// don't include repos that are created after gitea starts; such repos will
	// already be added to the indexer, and we don't need to add them again.
	for maxRepoID > 0 {
		repos := make([]*models.Repository, 0, models.RepositoryListDefaultPageSize)
		err = models.FindByMaxID(maxRepoID, models.RepositoryListDefaultPageSize, &repos)
		if err != nil {
			return err
		} else if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			codesIndexerQueue.Push(&IndexerData{
				RepoID:   repo.ID,
				Filepath: repo.RepoPath(),
				IsDelete: false,
			})
			maxRepoID = repo.ID - 1
		}
	}
	log.Info("Done populating the repo indexer with existing repositories")
	return nil
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
		return nil
	}

	fileContents, err := git.NewCommand("cat-file", "blob", update.BlobSha).
		RunInDirBytes(repo.RepoPath())
	if err != nil {
		return err
	} else if !base.IsTextFile(fileContents) {
		return nil
	}
	return batch.Index(
		filenameIndexerID(repo.ID, update.Filename),
		&IndexerData{
			RepoID:   repo.ID,
			Filepath: update.Filename,
			Content:  string(fileContents),
		},
	)
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
		DeleteRepoFromIndexer(repo)
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

// I assume we don't need this anymore because of the codeIndexerQueue

// func processRepoIndexerOperationQueue() {
// 	for {
// 		op := <-repoIndexerOperationQueue
// 		var err error
// 		if op.deleted {
// 			if err = indexer.DeleteRepoFromIndexer(op.repo.ID); err != nil {
// 				log.Error("DeleteRepoFromIndexer: %v", err)
// 			}
// 		} else {
// 			if err = updateRepoIndexer(op.repo); err != nil {
// 				log.Error("updateRepoIndexer: %v", err)
// 			}
// 		}
// 		for _, watcher := range op.watchers {
// 			watcher <- err
// 		}
// 	}
// }

// // DeleteRepoFromIndexer remove all of a repository's entries from the indexer
// func DeleteRepoFromIndexer(repo *Repository, watchers ...chan<- error) {
// 	addOperationToQueue(repoIndexerOperation{repo: repo, deleted: true, watchers: watchers})
// }

// // UpdateRepoIndexer update a repository's entries in the indexer
// func UpdateRepoIndexer(repo *Repository, watchers ...chan<- error) {
// 	addOperationToQueue(repoIndexerOperation{repo: repo, deleted: false, watchers: watchers})
// }

// func addOperationToQueue(op repoIndexerOperation) {
// 	if !setting.Indexer.RepoIndexerEnabled {
// 		return
// 	}
// 	select {
// 	case repoIndexerOperationQueue <- op:
// 		break
// 	default:
// 		go func() {
// 			repoIndexerOperationQueue <- op
// 		}()
// 	}
// }

// DeleteRepoFromIndexer remove all of a repository's entries from the indexer
func DeleteRepoFromIndexer(repo *models.Repository) {
	codesIndexerQueue.Push(&IndexerData{
		RepoID:   repo.ID,
		IsDelete: true,
	})
}

// UpdateRepoIndexer update a repository's entries in the indexer
func UpdateRepoIndexer(repo *models.Repository) {
	codesIndexerQueue.Push(&IndexerData{
		RepoID:   repo.ID,
		IsDelete: false,
	})
}
