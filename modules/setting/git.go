// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
)

var (
	// Git settings
	Git = struct {
		Version                  string `ini:"-"`
		DisableDiffHighlight     bool
		MaxGitDiffLines          int
		MaxGitDiffLineCharacters int
		MaxGitDiffFiles          int
		GCArgs                   []string `delim:" "`
		Timeout                  struct {
			Default int
			Migrate int
			Mirror  int
			Clone   int
			Pull    int
			GC      int `ini:"GC"`
		} `ini:"git.timeout"`
		LastCommitCache struct {
			UseDefaultCache bool
			Type            string
			ConnStr         string
		} `ini:"git.last_commit_cache"`
	}{
		DisableDiffHighlight:     false,
		MaxGitDiffLines:          1000,
		MaxGitDiffLineCharacters: 5000,
		MaxGitDiffFiles:          100,
		GCArgs:                   []string{},
		Timeout: struct {
			Default int
			Migrate int
			Mirror  int
			Clone   int
			Pull    int
			GC      int `ini:"GC"`
		}{
			Default: int(git.DefaultCommandExecutionTimeout / time.Second),
			Migrate: 600,
			Mirror:  300,
			Clone:   300,
			Pull:    300,
			GC:      60,
		},
		LastCommitCache: struct {
			UseDefaultCache bool
			Type            string
			ConnStr         string
		}{
			UseDefaultCache: false,
			Type:            "none",
			ConnStr:         "",
		},
	}
)

func newGitService() {
	if err := Cfg.Section("git").MapTo(&Git); err != nil {
		log.Fatal(4, "Failed to map Git settings: %v", err)
	}

	git.DefaultCommandExecutionTimeout = time.Duration(Git.Timeout.Default) * time.Second
}
