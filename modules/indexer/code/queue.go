// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"os"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type repoIndexerOperation struct {
	repoID   int64
	deleted  bool
	watchers []chan<- error
}

var repoIndexerOperationQueue chan repoIndexerOperation

func initQueue(queueLength int) {
	repoIndexerOperationQueue = make(chan repoIndexerOperation, queueLength)
}

func processRepoIndexerOperationQueue(indexer Indexer) {
	for {
		select {
		case op := <-repoIndexerOperationQueue:
			var err error
			if op.deleted {
				if err = indexer.Delete(op.repoID); err != nil {
					log.Error("indexer.Delete: %v", err)
				}
			} else {
				if err = indexer.Index(op.repoID); err != nil {
					log.Error("indexer.Index: %v", err)
				}
			}
			for _, watcher := range op.watchers {
				watcher <- err
			}
		case <-graceful.GetManager().IsShutdown():
			log.Info("PID: %d Repository indexer queue processing stopped", os.Getpid())
			return
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

// populateRepoIndexer populate the repo indexer with pre-existing data. This
// should only be run when the indexer is created for the first time.
func populateRepoIndexer() {
	log.Info("Populating the repo indexer with existing repositories")

	isShutdown := graceful.GetManager().IsShutdown()

	exist, err := models.IsTableNotEmpty("repository")
	if err != nil {
		log.Fatal("System error: %v", err)
	} else if !exist {
		return
	}

	// if there is any existing repo indexer metadata in the DB, delete it
	// since we are starting afresh. Also, xorm requires deletes to have a
	// condition, and we want to delete everything, thus 1=1.
	if err := models.DeleteAllRecords("repo_indexer_status"); err != nil {
		log.Fatal("System error: %v", err)
	}

	var maxRepoID int64
	if maxRepoID, err = models.GetMaxID("repository"); err != nil {
		log.Fatal("System error: %v", err)
	}

	// start with the maximum existing repo ID and work backwards, so that we
	// don't include repos that are created after gitea starts; such repos will
	// already be added to the indexer, and we don't need to add them again.
	for maxRepoID > 0 {
		select {
		case <-isShutdown:
			log.Info("Repository Indexer population shutdown before completion")
			return
		default:
		}
		ids, err := models.GetUnindexedRepos(maxRepoID, 0, 50)
		if err != nil {
			log.Error("populateRepoIndexer: %v", err)
			return
		} else if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			select {
			case <-isShutdown:
				log.Info("Repository Indexer population shutdown before completion")
				return
			default:
			}
			repoIndexerOperationQueue <- repoIndexerOperation{
				repoID:  id,
				deleted: false,
			}
			maxRepoID = id - 1
		}
	}
	log.Info("Done (re)populating the repo indexer with existing repositories")
}
