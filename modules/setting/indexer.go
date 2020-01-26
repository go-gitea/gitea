// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/gobwas/glob"
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
		StartupTimeout        time.Duration
		IncludePatterns       []glob.Glob
		ExcludePatterns       []glob.Glob
	}{
		IssueType:             "bleve",
		IssuePath:             "indexers/issues.bleve",
		IssueQueueType:        LevelQueueType,
		IssueQueueDir:         "indexers/issues.queue",
		IssueQueueConnStr:     "",
		IssueQueueBatchNumber: 20,

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
	Indexer.RepoIndexerEnabled = sec.Key("REPO_INDEXER_ENABLED").MustBool(false)
	Indexer.RepoPath = sec.Key("REPO_INDEXER_PATH").MustString(path.Join(AppDataPath, "indexers/repos.bleve"))
	if !filepath.IsAbs(Indexer.RepoPath) {
		Indexer.RepoPath = path.Join(AppWorkPath, Indexer.RepoPath)
	}
	Indexer.IncludePatterns = IndexerGlobFromString(sec.Key("REPO_INDEXER_INCLUDE").MustString(""))
	Indexer.ExcludePatterns = IndexerGlobFromString(sec.Key("REPO_INDEXER_EXCLUDE").MustString(""))

	Indexer.UpdateQueueLength = sec.Key("UPDATE_BUFFER_LEN").MustInt(20)
	Indexer.MaxIndexerFileSize = sec.Key("MAX_FILE_SIZE").MustInt64(1024 * 1024)
	Indexer.IssueQueueType = sec.Key("ISSUE_INDEXER_QUEUE_TYPE").MustString(LevelQueueType)
	Indexer.IssueQueueDir = sec.Key("ISSUE_INDEXER_QUEUE_DIR").MustString(path.Join(AppDataPath, "indexers/issues.queue"))
	Indexer.IssueQueueConnStr = sec.Key("ISSUE_INDEXER_QUEUE_CONN_STR").MustString(path.Join(AppDataPath, ""))
	Indexer.IssueQueueBatchNumber = sec.Key("ISSUE_INDEXER_QUEUE_BATCH_NUMBER").MustInt(20)
	Indexer.StartupTimeout = sec.Key("STARTUP_TIMEOUT").MustDuration(30 * time.Second)
}

// IndexerGlobFromString parses a comma separated list of patterns and returns a glob.Glob slice suited for repo indexing
func IndexerGlobFromString(globstr string) []glob.Glob {
	extarr := make([]glob.Glob, 0, 10)
	for _, expr := range strings.Split(strings.ToLower(globstr), ",") {
		expr = strings.TrimSpace(expr)
		if expr != "" {
			if g, err := glob.Compile(expr, '.', '/'); err != nil {
				log.Info("Invalid glob expresion '%s' (skipped): %v", expr, err)
			} else {
				extarr = append(extarr, g)
			}
		}
	}
	return extarr
}
