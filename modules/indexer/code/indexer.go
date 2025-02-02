// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"os"
	"runtime/pprof"
	"slices"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/indexer/code/bleve"
	"code.gitea.io/gitea/modules/indexer/code/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

var (
	indexerQueue *queue.WorkerPoolQueue[*internal.IndexerData]
	// globalIndexer is the global indexer, it cannot be nil.
	// When the real indexer is not ready, it will be a dummy indexer which will return error to explain it's not ready.
	// So it's always safe use it as *globalIndexer.Load() and call its methods.
	globalIndexer atomic.Pointer[internal.Indexer]
	dummyIndexer  *internal.Indexer
)

func init() {
	i := internal.NewDummyIndexer()
	dummyIndexer = &i
	globalIndexer.Store(dummyIndexer)
}

func index(ctx context.Context, indexer internal.Indexer, repoID int64) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if repo_model.IsErrRepoNotExist(err) {
		return indexer.Delete(ctx, repoID)
	}
	if err != nil {
		return err
	}

	repoTypes := setting.Indexer.RepoIndexerRepoTypes

	if len(repoTypes) == 0 {
		repoTypes = []string{"sources"}
	}

	// skip forks from being indexed if unit is not present
	if !slices.Contains(repoTypes, "forks") && repo.IsFork {
		return nil
	}

	// skip mirrors from being indexed if unit is not present
	if !slices.Contains(repoTypes, "mirrors") && repo.IsMirror {
		return nil
	}

	// skip templates from being indexed if unit is not present
	if !slices.Contains(repoTypes, "templates") && repo.IsTemplate {
		return nil
	}

	// skip regular repos from being indexed if unit is not present
	if !slices.Contains(repoTypes, "sources") && !repo.IsFork && !repo.IsMirror && !repo.IsTemplate {
		return nil
	}

	sha, err := getDefaultBranchSha(ctx, repo)
	if err != nil {
		return err
	}
	changes, err := getRepoChanges(ctx, repo, sha)
	if err != nil {
		return err
	} else if changes == nil {
		return nil
	}

	if err := indexer.Index(ctx, repo, sha, changes); err != nil {
		return err
	}

	return repo_model.UpdateIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeCode, sha)
}

// Init initialize the repo indexer
func Init() {
	if !setting.Indexer.RepoIndexerEnabled {
		(*globalIndexer.Load()).Close()
		return
	}

	ctx, cancel, finished := process.GetManager().AddTypedContext(context.Background(), "Service: CodeIndexer", process.SystemProcessType, false)

	graceful.GetManager().RunAtTerminate(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		cancel()
		log.Debug("Closing repository indexer")
		(*globalIndexer.Load()).Close()
		log.Info("PID: %d Repository Indexer closed", os.Getpid())
		finished()
	})

	waitChannel := make(chan time.Duration, 1)

	// Create the Queue
	switch setting.Indexer.RepoType {
	case "bleve", "elasticsearch":
		handler := func(items ...*internal.IndexerData) (unhandled []*internal.IndexerData) {
			indexer := *globalIndexer.Load()
			for _, indexerData := range items {
				log.Trace("IndexerData Process Repo: %d", indexerData.RepoID)
				if err := index(ctx, indexer, indexerData.RepoID); err != nil {
					if !setting.IsInTesting {
						log.Error("Codes indexer handler: index error for repo %v: %v", indexerData.RepoID, err)
					}
				}
			}
			return nil // do not re-queue the failed items, otherwise some broken repo will block the queue
		}

		indexerQueue = queue.CreateUniqueQueue(ctx, "code_indexer", handler)
		if indexerQueue == nil {
			log.Fatal("Unable to create codes indexer queue")
		}
	default:
		log.Fatal("Unknown codes indexer type; %s", setting.Indexer.RepoType)
	}

	go func() {
		pprof.SetGoroutineLabels(ctx)
		start := time.Now()
		var (
			rIndexer internal.Indexer
			existed  bool
			err      error
		)
		switch setting.Indexer.RepoType {
		case "bleve":
			log.Info("PID: %d Initializing Repository Indexer at: %s", os.Getpid(), setting.Indexer.RepoPath)
			defer func() {
				if err := recover(); err != nil {
					log.Error("PANIC whilst initializing repository indexer: %v\nStacktrace: %s", err, log.Stack(2))
					log.Error("The indexer files are likely corrupted and may need to be deleted")
					log.Error("You can completely remove the \"%s\" directory to make Gitea recreate the indexes", setting.Indexer.RepoPath)
				}
			}()

			rIndexer = bleve.NewIndexer(setting.Indexer.RepoPath)
			existed, err = rIndexer.Init(ctx)
			if err != nil {
				cancel()
				(*globalIndexer.Load()).Close()
				close(waitChannel)
				log.Fatal("PID: %d Unable to initialize the bleve Repository Indexer at path: %s Error: %v", os.Getpid(), setting.Indexer.RepoPath, err)
			}
		case "elasticsearch":
			log.Info("PID: %d Initializing Repository Indexer at: %s", os.Getpid(), setting.Indexer.RepoConnStr)
			defer func() {
				if err := recover(); err != nil {
					log.Error("PANIC whilst initializing repository indexer: %v\nStacktrace: %s", err, log.Stack(2))
					log.Error("The indexer files are likely corrupted and may need to be deleted")
					log.Error("You can completely remove the \"%s\" index to make Gitea recreate the indexes", setting.Indexer.RepoConnStr)
				}
			}()

			rIndexer = elasticsearch.NewIndexer(setting.Indexer.RepoConnStr, setting.Indexer.RepoIndexerName)
			existed, err = rIndexer.Init(ctx)
			if err != nil {
				cancel()
				(*globalIndexer.Load()).Close()
				close(waitChannel)
				log.Fatal("PID: %d Unable to initialize the elasticsearch Repository Indexer connstr: %s Error: %v", os.Getpid(), setting.Indexer.RepoConnStr, err)
			}

		default:
			log.Fatal("PID: %d Unknown Indexer type: %s", os.Getpid(), setting.Indexer.RepoType)
		}

		globalIndexer.Store(&rIndexer)

		// Start processing the queue
		go graceful.GetManager().RunWithCancel(indexerQueue)

		if !existed { // populate the index because it's created for the first time
			go graceful.GetManager().RunWithShutdownContext(populateRepoIndexer)
		}
		select {
		case waitChannel <- time.Since(start):
		case <-graceful.GetManager().IsShutdown():
		}

		close(waitChannel)
	}()

	if setting.Indexer.StartupTimeout > 0 {
		go func() {
			pprof.SetGoroutineLabels(ctx)
			timeout := setting.Indexer.StartupTimeout
			if graceful.GetManager().IsChild() && setting.GracefulHammerTime > 0 {
				timeout += setting.GracefulHammerTime
			}
			select {
			case <-graceful.GetManager().IsShutdown():
				log.Warn("Shutdown before Repository Indexer completed initialization")
				cancel()
				(*globalIndexer.Load()).Close()
			case duration, ok := <-waitChannel:
				if !ok {
					log.Warn("Repository Indexer Initialization failed")
					cancel()
					(*globalIndexer.Load()).Close()
					return
				}
				log.Info("Repository Indexer Initialization took %v", duration)
			case <-time.After(timeout):
				cancel()
				(*globalIndexer.Load()).Close()
				log.Fatal("Repository Indexer Initialization Timed-Out after: %v", timeout)
			}
		}()
	}
}

// UpdateRepoIndexer update a repository's entries in the indexer
func UpdateRepoIndexer(repo *repo_model.Repository) {
	indexData := &internal.IndexerData{RepoID: repo.ID}
	if err := indexerQueue.Push(indexData); err != nil {
		log.Error("Update repo index data %v failed: %v", indexData, err)
	}
}

// IsAvailable checks if issue indexer is available
func IsAvailable(ctx context.Context) bool {
	return (*globalIndexer.Load()).Ping(ctx) == nil
}

// populateRepoIndexer populate the repo indexer with pre-existing data. This
// should only be run when the indexer is created for the first time.
func populateRepoIndexer(ctx context.Context) {
	log.Info("Populating the repo indexer with existing repositories")

	exist, err := db.IsTableNotEmpty("repository")
	if err != nil {
		log.Fatal("System error: %v", err)
	} else if !exist {
		return
	}

	// if there is any existing repo indexer metadata in the DB, delete it
	// since we are starting afresh. Also, xorm requires deletes to have a
	// condition, and we want to delete everything, thus 1=1.
	if err := db.DeleteAllRecords("repo_indexer_status"); err != nil {
		log.Fatal("System error: %v", err)
	}

	var maxRepoID int64
	if maxRepoID, err = db.GetMaxID("repository"); err != nil {
		log.Fatal("System error: %v", err)
	}

	// start with the maximum existing repo ID and work backwards, so that we
	// don't include repos that are created after gitea starts; such repos will
	// already be added to the indexer, and we don't need to add them again.
	for maxRepoID > 0 {
		select {
		case <-ctx.Done():
			log.Info("Repository Indexer population shutdown before completion")
			return
		default:
		}
		ids, err := repo_model.GetUnindexedRepos(ctx, repo_model.RepoIndexerTypeCode, maxRepoID, 0, 50)
		if err != nil {
			log.Error("populateRepoIndexer: %v", err)
			return
		} else if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			select {
			case <-ctx.Done():
				log.Info("Repository Indexer population shutdown before completion")
				return
			default:
			}
			if err := indexerQueue.Push(&internal.IndexerData{RepoID: id}); err != nil {
				log.Error("indexerQueue.Push: %v", err)
				return
			}
			maxRepoID = id - 1
		}
	}
	log.Info("Done (re)populating the repo indexer with existing repositories")
}
