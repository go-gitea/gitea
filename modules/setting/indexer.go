// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path"
	"path/filepath"
	"strings"
)

// enumerates all the indexer queue types
const (
	LevelQueueType   = "levelqueue"
	ChannelQueueType = "channel"
	RedisQueueType   = "redis"
)

var (
	// Indexer settings
	Indexer = struct {
		IssueType             string
		IssuePath             string
		RepoIndexerEnabled    bool
		RepoPath              string
		UpdateQueueLength     int
		MaxIndexerFileSize    int64
		IssueQueueType        string
		IssueQueueDir         string
		IssueQueueConnStr     string
		IssueQueueBatchNumber int
		FileExtensions        map[string]bool
		ExcludeExtensions     bool
	}{
		IssueType:             "bleve",
		IssuePath:             "indexers/issues.bleve",
		IssueQueueType:        LevelQueueType,
		IssueQueueDir:         "indexers/issues.queue",
		IssueQueueConnStr:     "",
		IssueQueueBatchNumber: 20,
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
	Indexer.FileExtensions = extensionsFromString(sec.Key("REPO_INDEXER_EXTENSIONS").MustString(""))
	Indexer.ExcludeExtensions = sec.Key("REPO_EXTENSIONS_LIST_EXCLUDE").MustBool(false)

	Indexer.UpdateQueueLength = sec.Key("UPDATE_BUFFER_LEN").MustInt(20)
	Indexer.MaxIndexerFileSize = sec.Key("MAX_FILE_SIZE").MustInt64(1024 * 1024)
	Indexer.IssueQueueType = sec.Key("ISSUE_INDEXER_QUEUE_TYPE").MustString(LevelQueueType)
	Indexer.IssueQueueDir = sec.Key("ISSUE_INDEXER_QUEUE_DIR").MustString(path.Join(AppDataPath, "indexers/issues.queue"))
	Indexer.IssueQueueConnStr = sec.Key("ISSUE_INDEXER_QUEUE_CONN_STR").MustString(path.Join(AppDataPath, ""))
	Indexer.IssueQueueBatchNumber = sec.Key("ISSUE_INDEXER_QUEUE_BATCH_NUMBER").MustInt(20)
}

func extensionsFromString(from string) map[string]bool {
	extmap := make(map[string]bool)
	for _, ext := range strings.Split(strings.ToLower(from), ",") {
		ext = strings.TrimSpace(ext)
		if ext == "." {
			extmap[""] = true
		} else if ext != "" && !strings.Contains(ext, ".") {
			extmap[ext] = true
		}
	}
	if len(extmap) == 0 {
		return nil
	}
	return extmap
}
