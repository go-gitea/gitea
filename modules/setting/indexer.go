// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/gobwas/glob"
)

// Indexer settings
var Indexer = struct {
	IssueType        string
	IssuePath        string
	IssueConnStr     string
	IssueIndexerName string
	StartupTimeout   time.Duration

	RepoIndexerEnabled bool
	RepoType           string
	RepoPath           string
	RepoConnStr        string
	RepoIndexerName    string
	MaxIndexerFileSize int64
	IncludePatterns    []glob.Glob
	ExcludePatterns    []glob.Glob
	ExcludeVendored    bool
}{
	IssueType:        "bleve",
	IssuePath:        "indexers/issues.bleve",
	IssueConnStr:     "",
	IssueIndexerName: "gitea_issues",

	RepoIndexerEnabled: false,
	RepoType:           "bleve",
	RepoPath:           "indexers/repos.bleve",
	RepoConnStr:        "",
	RepoIndexerName:    "gitea_codes",
	MaxIndexerFileSize: 1024 * 1024,
	ExcludeVendored:    true,
}

func newIndexerService() {
	sec := Cfg.Section("indexer")
	Indexer.IssueType = sec.Key("ISSUE_INDEXER_TYPE").MustString("bleve")
	Indexer.IssuePath = filepath.ToSlash(sec.Key("ISSUE_INDEXER_PATH").MustString(filepath.ToSlash(filepath.Join(AppDataPath, "indexers/issues.bleve"))))
	if !filepath.IsAbs(Indexer.IssuePath) {
		Indexer.IssuePath = filepath.ToSlash(filepath.Join(AppWorkPath, Indexer.IssuePath))
	}
	Indexer.IssueConnStr = sec.Key("ISSUE_INDEXER_CONN_STR").MustString(Indexer.IssueConnStr)
	Indexer.IssueIndexerName = sec.Key("ISSUE_INDEXER_NAME").MustString(Indexer.IssueIndexerName)

	// The following settings are deprecated and can be overridden by settings in [queue] or [queue.issue_indexer]
	// FIXME: DEPRECATED to be removed in v1.18.0
	deprecatedSetting("indexer", "ISSUE_INDEXER_QUEUE_TYPE", "queue.issue_indexer", "TYPE")
	deprecatedSetting("indexer", "ISSUE_INDEXER_QUEUE_DIR", "queue.issue_indexer", "DATADIR")
	deprecatedSetting("indexer", "ISSUE_INDEXER_QUEUE_CONN_STR", "queue.issue_indexer", "CONN_STR")
	deprecatedSetting("indexer", "ISSUE_INDEXER_QUEUE_BATCH_NUMBER", "queue.issue_indexer", "BATCH_LENGTH")
	deprecatedSetting("indexer", "UPDATE_BUFFER_LEN", "queue.issue_indexer", "LENGTH")

	Indexer.RepoIndexerEnabled = sec.Key("REPO_INDEXER_ENABLED").MustBool(false)
	Indexer.RepoType = sec.Key("REPO_INDEXER_TYPE").MustString("bleve")
	Indexer.RepoPath = filepath.ToSlash(sec.Key("REPO_INDEXER_PATH").MustString(filepath.ToSlash(filepath.Join(AppDataPath, "indexers/repos.bleve"))))
	if !filepath.IsAbs(Indexer.RepoPath) {
		Indexer.RepoPath = filepath.ToSlash(filepath.Join(AppWorkPath, Indexer.RepoPath))
	}
	Indexer.RepoConnStr = sec.Key("REPO_INDEXER_CONN_STR").MustString("")
	Indexer.RepoIndexerName = sec.Key("REPO_INDEXER_NAME").MustString("gitea_codes")

	Indexer.IncludePatterns = IndexerGlobFromString(sec.Key("REPO_INDEXER_INCLUDE").MustString(""))
	Indexer.ExcludePatterns = IndexerGlobFromString(sec.Key("REPO_INDEXER_EXCLUDE").MustString(""))
	Indexer.ExcludeVendored = sec.Key("REPO_INDEXER_EXCLUDE_VENDORED").MustBool(true)
	Indexer.MaxIndexerFileSize = sec.Key("MAX_FILE_SIZE").MustInt64(1024 * 1024)
	Indexer.StartupTimeout = sec.Key("STARTUP_TIMEOUT").MustDuration(30 * time.Second)
}

// IndexerGlobFromString parses a comma separated list of patterns and returns a glob.Glob slice suited for repo indexing
func IndexerGlobFromString(globstr string) []glob.Glob {
	extarr := make([]glob.Glob, 0, 10)
	for _, expr := range strings.Split(strings.ToLower(globstr), ",") {
		expr = strings.TrimSpace(expr)
		if expr != "" {
			if g, err := glob.Compile(expr, '.', '/'); err != nil {
				log.Info("Invalid glob expression '%s' (skipped): %v", expr, err)
			} else {
				extarr = append(extarr, g)
			}
		}
	}
	return extarr
}
