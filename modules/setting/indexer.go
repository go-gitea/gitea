// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// Indexer settings
var Indexer = struct {
	IssueType        string
	IssuePath        string
	IssueConnStr     string
	IssueConnAuth    string
	IssueIndexerName string
	StartupTimeout   time.Duration

	RepoIndexerEnabled   bool
	RepoIndexerRepoTypes []string
	RepoType             string
	RepoPath             string
	RepoConnStr          string
	RepoIndexerName      string
	MaxIndexerFileSize   int64
	IncludePatterns      []*GlobMatcher
	ExcludePatterns      []*GlobMatcher
	ExcludeVendored      bool
}{
	IssueType:        "bleve",
	IssuePath:        "indexers/issues.bleve",
	IssueConnStr:     "",
	IssueConnAuth:    "",
	IssueIndexerName: "gitea_issues",

	RepoIndexerEnabled:   false,
	RepoIndexerRepoTypes: []string{"sources", "forks", "mirrors", "templates"},
	RepoType:             "bleve",
	RepoPath:             "indexers/repos.bleve",
	RepoConnStr:          "",
	RepoIndexerName:      "gitea_codes",
	MaxIndexerFileSize:   1024 * 1024,
	ExcludeVendored:      true,
}

func loadIndexerFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("indexer")
	Indexer.IssueType = sec.Key("ISSUE_INDEXER_TYPE").MustString("bleve")
	if Indexer.IssueType == "bleve" {
		Indexer.IssuePath = filepath.ToSlash(sec.Key("ISSUE_INDEXER_PATH").MustString(filepath.ToSlash(filepath.Join(AppDataPath, "indexers/issues.bleve"))))
		if !filepath.IsAbs(Indexer.IssuePath) {
			Indexer.IssuePath = filepath.ToSlash(filepath.Join(AppWorkPath, Indexer.IssuePath))
		}
		checkOverlappedPath("[indexer].ISSUE_INDEXER_PATH", Indexer.IssuePath)
	} else {
		Indexer.IssueConnStr = sec.Key("ISSUE_INDEXER_CONN_STR").MustString(Indexer.IssueConnStr)
		if Indexer.IssueType == "meilisearch" {
			u, err := url.Parse(Indexer.IssueConnStr)
			if err != nil {
				log.Warn("Failed to parse ISSUE_INDEXER_CONN_STR: %v", err)
				u = &url.URL{}
			}
			Indexer.IssueConnAuth, _ = u.User.Password()
			u.User = nil
			Indexer.IssueConnStr = u.String()
		}
	}

	Indexer.IssueIndexerName = sec.Key("ISSUE_INDEXER_NAME").MustString(Indexer.IssueIndexerName)

	Indexer.RepoIndexerEnabled = sec.Key("REPO_INDEXER_ENABLED").MustBool(false)
	Indexer.RepoIndexerRepoTypes = strings.Split(sec.Key("REPO_INDEXER_REPO_TYPES").MustString("sources,forks,mirrors,templates"), ",")
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
func IndexerGlobFromString(globstr string) []*GlobMatcher {
	extarr := make([]*GlobMatcher, 0, 10)
	for _, expr := range strings.Split(strings.ToLower(globstr), ",") {
		expr = strings.TrimSpace(expr)
		if expr != "" {
			if g, err := GlobMatcherCompile(expr, '.', '/'); err != nil {
				log.Info("Invalid glob expression '%s' (skipped): %v", expr, err)
			} else {
				extarr = append(extarr, g)
			}
		}
	}
	return extarr
}
