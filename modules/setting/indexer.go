// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path"
	"path/filepath"
)

// enumerates all the indexer queue types
const (
	LevelQueueType   = "levelqueue"
	ChannelQueueType = "channel"
)

var (
	// Indexer settings
	Indexer = struct {
		IssueType                    string
		IssuePath                    string
		RepoIndexerEnabled           bool
		RepoPath                     string
		UpdateQueueLength            int
		MaxIndexerFileSize           int64
		IssueIndexerQueueType        string
		IssueIndexerQueueDir         string
		IssueIndexerQueueBatchNumber int
	}{
		IssueType:                    "bleve",
		IssuePath:                    "indexers/issues.bleve",
		IssueIndexerQueueType:        LevelQueueType,
		IssueIndexerQueueDir:         "indexers/issues.queue",
		IssueIndexerQueueBatchNumber: 20,
	}
)

func newIndexerService() {
	sec := Cfg.Section("indexer")
	Indexer.IssueType = sec.Key("ISSUE_INDEXER_TYPE").MustString("bleve")
	Indexer.IssuePath = sec.Key("ISSUE_INDEXER_PATH").MustString(path.Join(AppDataPath, "indexers/issues.bleve"))
	if !filepath.IsAbs(Indexer.IssuePath) {
		Indexer.IssuePath = path.Join(AppWorkPath, Indexer.IssuePath)
	}
	Indexer.RepoIndexerEnabled = sec.Key("REPO_INDEXER_ENABLED").MustBool(false)
	Indexer.RepoPath = sec.Key("REPO_INDEXER_PATH").MustString(path.Join(AppDataPath, "indexers/repos.bleve"))
	if !filepath.IsAbs(Indexer.RepoPath) {
		Indexer.RepoPath = path.Join(AppWorkPath, Indexer.RepoPath)
	}
	Indexer.UpdateQueueLength = sec.Key("UPDATE_BUFFER_LEN").MustInt(20)
	Indexer.MaxIndexerFileSize = sec.Key("MAX_FILE_SIZE").MustInt64(1024 * 1024)
	Indexer.IssueIndexerQueueType = sec.Key("ISSUE_INDEXER_QUEUE_TYPE").MustString(LevelQueueType)
	Indexer.IssueIndexerQueueDir = sec.Key("ISSUE_INDEXER_QUEUE_DIR").MustString(path.Join(AppDataPath, "indexers/issues.queue"))
	Indexer.IssueIndexerQueueBatchNumber = sec.Key("ISSUE_INDEXER_QUEUE_BATCH_NUMBER").MustInt(20)
}
