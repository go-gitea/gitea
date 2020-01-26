// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"context"
	"os"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// SearchResult result of performing a search in a repo
type SearchResult struct {
	RepoID     int64
	StartIndex int
	EndIndex   int
	Filename   string
	Content    string
}

// Indexer defines an interface to indexer issues contents
type Indexer interface {
	Index(repoID int64) error
	Delete(repoID int64) error
	Search(repoIDs []int64, keyword string, page, pageSize int) (int64, []*SearchResult, error)
	Close()
}

// Init initialize the repo indexer
func Init() {
	if !setting.Indexer.RepoIndexerEnabled {
		indexer.Close()
		return
	}

	initQueue(setting.Indexer.UpdateQueueLength)

	ctx, cancel := context.WithCancel(context.Background())

	graceful.GetManager().RunAtTerminate(ctx, func() {
		log.Debug("Closing repository indexer")
		indexer.Close()
		log.Info("PID: %d Repository Indexer closed", os.Getpid())
	})

	waitChannel := make(chan time.Duration)
	go func() {
		start := time.Now()
		log.Info("PID: %d Initializing Repository Indexer at: %s", os.Getpid(), setting.Indexer.RepoPath)
		bleveIndexer, created, err := NewBleveIndexer(setting.Indexer.RepoPath)
		if err != nil {
			if bleveIndexer != nil {
				bleveIndexer.Close()
			}
			cancel()
			indexer.Close()
			close(waitChannel)
			log.Fatal("PID: %d Unable to initialize the Repository Indexer at path: %s Error: %v", os.Getpid(), setting.Indexer.RepoPath, err)
		}
		indexer.set(bleveIndexer)

		go processRepoIndexerOperationQueue(indexer)

		if created {
			go populateRepoIndexer()
		}
		select {
		case waitChannel <- time.Since(start):
		case <-graceful.GetManager().IsShutdown():
		}

		close(waitChannel)
	}()

	if setting.Indexer.StartupTimeout > 0 {
		go func() {
			timeout := setting.Indexer.StartupTimeout
			if graceful.GetManager().IsChild() && setting.GracefulHammerTime > 0 {
				timeout += setting.GracefulHammerTime
			}
			select {
			case <-graceful.GetManager().IsShutdown():
				log.Warn("Shutdown before Repository Indexer completed initialization")
				cancel()
				indexer.Close()
			case duration, ok := <-waitChannel:
				if !ok {
					log.Warn("Repository Indexer Initialization failed")
					cancel()
					indexer.Close()
					return
				}
				log.Info("Repository Indexer Initialization took %v", duration)
			case <-time.After(timeout):
				cancel()
				indexer.Close()
				log.Fatal("Repository Indexer Initialization Timed-Out after: %v", timeout)
			}
		}()
	}
}
