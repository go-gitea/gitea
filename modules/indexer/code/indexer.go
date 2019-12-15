// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var (
	indexer Indexer
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
	Init() (bool, error)
	Index(repoID int64) error
	Delete(repoID int64) error
	Search(repoIDs []int64, keyword string, page, pageSize int) (int64, []*SearchResult, error)
}

// Init initialize the repo indexer
func Init() {
	if !setting.Indexer.RepoIndexerEnabled {
		return
	}

	waitChannel := make(chan time.Duration)
	go func() {
		start := time.Now()
		log.Info("Initializing Repository Indexer")
		indexer = NewBleveIndexer(setting.Indexer.RepoPath)
		created, err := indexer.Init()
		if err != nil {
			log.Fatal("indexer.Init: %v", err)
		}

		go processRepoIndexerOperationQueue()

		if created {
			go populateRepoIndexer()
		}

		waitChannel <- time.Since(start)
	}()

	if setting.Indexer.StartupTimeout > 0 {
		go func() {
			timeout := setting.Indexer.StartupTimeout
			if graceful.GetManager().IsChild() && setting.GracefulHammerTime > 0 {
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
