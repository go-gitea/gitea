// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"github.com/Unknwon/com"
)

// enumerates all the policy repository creating
const (
	RepoCreatingLastUserVisibility = "last"
	RepoCreatingPrivate            = "private"
	RepoCreatingPublic             = "public"
)

// Repository settings
var (
	Repository = struct {
		AnsiCharset                             string
		ForcePrivate                            bool
		DefaultPrivate                          string
		MaxCreationLimit                        int
		MirrorQueueLength                       int
		PullRequestQueueLength                  int
		PreferredLicenses                       []string
		DisableHTTPGit                          bool
		AccessControlAllowOrigin                string
		UseCompatSSHURI                         bool
		DefaultCloseIssuesViaCommitsInAnyBranch bool

		// Repository editor settings
		Editor struct {
			LineWrapExtensions   []string
			PreviewableFileModes []string
		} `ini:"-"`

		// Repository upload settings
		Upload struct {
			Enabled      bool
			TempPath     string
			AllowedTypes []string `delim:"|"`
			FileMaxSize  int64
			MaxFiles     int
		} `ini:"-"`

		// Repository local settings
		Local struct {
			LocalCopyPath string
			LocalWikiPath string
		} `ini:"-"`

		// Pull request settings
		PullRequest struct {
			WorkInProgressPrefixes []string
		} `ini:"repository.pull-request"`

		// Issue Setting
		Issue struct {
			LockReasons []string
		} `ini:"repository.issue"`
	}{
		AnsiCharset:                             "",
		ForcePrivate:                            false,
		DefaultPrivate:                          RepoCreatingLastUserVisibility,
		MaxCreationLimit:                        -1,
		MirrorQueueLength:                       1000,
		PullRequestQueueLength:                  1000,
		PreferredLicenses:                       []string{"Apache License 2.0,MIT License"},
		DisableHTTPGit:                          false,
		AccessControlAllowOrigin:                "",
		UseCompatSSHURI:                         false,
		DefaultCloseIssuesViaCommitsInAnyBranch: false,

		// Repository editor settings
		Editor: struct {
			LineWrapExtensions   []string
			PreviewableFileModes []string
		}{
			LineWrapExtensions:   strings.Split(".txt,.md,.markdown,.mdown,.mkd,", ","),
			PreviewableFileModes: []string{"markdown"},
		},

		// Repository upload settings
		Upload: struct {
			Enabled      bool
			TempPath     string
			AllowedTypes []string `delim:"|"`
			FileMaxSize  int64
			MaxFiles     int
		}{
			Enabled:      true,
			TempPath:     "data/tmp/uploads",
			AllowedTypes: []string{},
			FileMaxSize:  3,
			MaxFiles:     5,
		},

		// Repository local settings
		Local: struct {
			LocalCopyPath string
			LocalWikiPath string
		}{
			LocalCopyPath: "tmp/local-repo",
			LocalWikiPath: "tmp/local-wiki",
		},

		// Pull request settings
		PullRequest: struct {
			WorkInProgressPrefixes []string
		}{
			WorkInProgressPrefixes: []string{"WIP:", "[WIP]"},
		},

		// Issue settings
		Issue: struct {
			LockReasons []string
		}{
			LockReasons: strings.Split("Too heated,Off-topic,Spam,Resolved", ","),
		},
	}
	RepoRootPath string
	ScriptType   = "bash"
)

func newRepository() {
	homeDir, err := com.HomeDir()
	if err != nil {
		log.Fatal(4, "Failed to get home directory: %v", err)
	}
	homeDir = strings.Replace(homeDir, "\\", "/", -1)

	// Determine and create root git repository path.
	sec := Cfg.Section("repository")
	Repository.DisableHTTPGit = sec.Key("DISABLE_HTTP_GIT").MustBool()
	Repository.UseCompatSSHURI = sec.Key("USE_COMPAT_SSH_URI").MustBool()
	Repository.MaxCreationLimit = sec.Key("MAX_CREATION_LIMIT").MustInt(-1)
	RepoRootPath = sec.Key("ROOT").MustString(path.Join(homeDir, "gitea-repositories"))
	forcePathSeparator(RepoRootPath)
	if !filepath.IsAbs(RepoRootPath) {
		RepoRootPath = filepath.Join(AppWorkPath, RepoRootPath)
	} else {
		RepoRootPath = filepath.Clean(RepoRootPath)
	}
	ScriptType = sec.Key("SCRIPT_TYPE").MustString("bash")

	if err = Cfg.Section("repository").MapTo(&Repository); err != nil {
		log.Fatal(4, "Failed to map Repository settings: %v", err)
	} else if err = Cfg.Section("repository.editor").MapTo(&Repository.Editor); err != nil {
		log.Fatal(4, "Failed to map Repository.Editor settings: %v", err)
	} else if err = Cfg.Section("repository.upload").MapTo(&Repository.Upload); err != nil {
		log.Fatal(4, "Failed to map Repository.Upload settings: %v", err)
	} else if err = Cfg.Section("repository.local").MapTo(&Repository.Local); err != nil {
		log.Fatal(4, "Failed to map Repository.Local settings: %v", err)
	} else if err = Cfg.Section("repository.pull-request").MapTo(&Repository.PullRequest); err != nil {
		log.Fatal(4, "Failed to map Repository.PullRequest settings: %v", err)
	}

	if !filepath.IsAbs(Repository.Upload.TempPath) {
		Repository.Upload.TempPath = path.Join(AppWorkPath, Repository.Upload.TempPath)
	}
}
