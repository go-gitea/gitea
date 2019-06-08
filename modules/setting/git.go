// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
	version "github.com/mcuadros/go-version"
)

var (
	// Git settings
	Git = struct {
		Version                  string `ini:"-"`
		DisableDiffHighlight     bool
		MaxGitDiffLines          int
		MaxGitDiffLineCharacters int
		MaxGitDiffFiles          int
		GCArgs                   []string `ini:"GC_ARGS" delim:" "`
		Timeout                  struct {
			Default int
			Migrate int
			Mirror  int
			Clone   int
			Pull    int
			GC      int `ini:"GC"`
		} `ini:"git.timeout"`
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
	}
)

func newGit() {
	if err := Cfg.Section("git").MapTo(&Git); err != nil {
		log.Fatal(4, "Failed to map Git settings: %v", err)
	}
	git.DefaultCommandExecutionTimeout = time.Duration(Git.Timeout.Default) * time.Second

	binVersion, err := git.BinVersion()
	if err != nil {
		log.Fatal(4, "Error retrieving git version: %v", err)
	}

	if version.Compare(binVersion, "2.9", ">=") {
		// Explicitly disable credential helper, otherwise Git credentials might leak
		git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "credential.helper=")
	}
}
