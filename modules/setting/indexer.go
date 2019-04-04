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
	RedisQueueType   = "redis"
	NoQueueType      = "noqueue"
)

var (
	// Indexer settings
	Indexer = struct {
		IssueType                string
		IssuePath                string
		IssueConnStr             string
		IssueESCustomMappingPath string

		IssueQueueType        string
		IssueQueueDir         string
		IssueQueueConnStr     string
		IssueQueueBatchNumber int

		RepoIndexerEnabled bool
		RepoPath           string
		UpdateQueueLength  int
		MaxIndexerFileSize int64
	}{
		IssueType:                "bleve",
		IssuePath:                "indexers/issues.bleve",
		IssueConnStr:             "",
		IssueESCustomMappingPath: "",

		IssueQueueType:        LevelQueueType,
		IssueQueueDir:         "indexers/issues.queue",
		IssueQueueConnStr:     "",
		IssueQueueBatchNumber: 20,

		RepoIndexerEnabled: false,
		RepoPath:           "indexers/repos.bleve",
		UpdateQueueLength:  20,
		MaxIndexerFileSize: 1024 * 1024,
	}
)

func newIndexerService() {
	sec := Cfg.Section("indexer")
	Indexer.IssueType = sec.Key("ISSUE_INDEXER_TYPE").MustString("bleve")
	Indexer.IssuePath = sec.Key("ISSUE_INDEXER_PATH").MustString(path.Join(AppDataPath, "indexers/issues.bleve"))
	if !filepath.IsAbs(Indexer.IssuePath) {
		Indexer.IssuePath = path.Join(AppWorkPath, Indexer.IssuePath)
	}
	Indexer.IssueConnStr = sec.Key("ISSUE_INDEXER_CONN_STR").MustString("")
	Indexer.IssueESCustomMappingPath = sec.Key("ISSUE_ES_CUSTOM_MAPPING_PATH").MustString("")

	Indexer.IssueQueueType = sec.Key("ISSUE_INDEXER_QUEUE_TYPE").MustString(LevelQueueType)
	Indexer.IssueQueueDir = sec.Key("ISSUE_INDEXER_QUEUE_DIR").MustString(path.Join(AppDataPath, "indexers/issues.queue"))
	Indexer.IssueQueueConnStr = sec.Key("ISSUE_INDEXER_QUEUE_CONN_STR").MustString(path.Join(AppDataPath, ""))
	Indexer.IssueQueueBatchNumber = sec.Key("ISSUE_INDEXER_QUEUE_BATCH_NUMBER").MustInt(20)

	Indexer.RepoIndexerEnabled = sec.Key("REPO_INDEXER_ENABLED").MustBool(false)
	Indexer.RepoPath = sec.Key("REPO_INDEXER_PATH").MustString(path.Join(AppDataPath, "indexers/repos.bleve"))
	if !filepath.IsAbs(Indexer.RepoPath) {
		Indexer.RepoPath = path.Join(AppWorkPath, Indexer.RepoPath)
	}
	Indexer.UpdateQueueLength = sec.Key("UPDATE_BUFFER_LEN").MustInt(20)
	Indexer.MaxIndexerFileSize = sec.Key("MAX_FILE_SIZE").MustInt64(1024 * 1024)
}
