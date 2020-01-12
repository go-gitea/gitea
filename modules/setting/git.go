// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	version "github.com/mcuadros/go-version"
)

var (
	// Git settings
	Git = struct {
		Path                      string
		DisableDiffHighlight      bool
		MaxGitDiffLines           int
		MaxGitDiffLineCharacters  int
		MaxGitDiffFiles           int
		VerbosePush               bool
		VerbosePushDelay          time.Duration
		GCArgs                    []string `ini:"GC_ARGS" delim:" "`
		EnableAutoGitWireProtocol bool
		Timeout                   struct {
			Default int
			Migrate int
			Mirror  int
			Clone   int
			Pull    int
			GC      int `ini:"GC"`
		} `ini:"git.timeout"`
	}{
		DisableDiffHighlight:      false,
		MaxGitDiffLines:           1000,
		MaxGitDiffLineCharacters:  5000,
		MaxGitDiffFiles:           100,
		VerbosePush:               true,
		VerbosePushDelay:          5 * time.Second,
		GCArgs:                    []string{},
		EnableAutoGitWireProtocol: true,
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
		log.Fatal("Failed to map Git settings: %v", err)
	}
	if err := git.SetExecutablePath(Git.Path); err != nil {
		log.Fatal("Failed to initialize Git settings", err)
	}
	git.DefaultCommandExecutionTimeout = time.Duration(Git.Timeout.Default) * time.Second

	binVersion, err := git.BinVersion()
	if err != nil {
		log.Fatal("Error retrieving git version: %v", err)
	}

	if version.Compare(binVersion, "2.9", ">=") {
		// Explicitly disable credential helper, otherwise Git credentials might leak
		git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "credential.helper=")
	}

	var format = "Git Version: %s"
	var args = []interface{}{binVersion}
	// Since git wire protocol has been released from git v2.18
	if Git.EnableAutoGitWireProtocol && version.Compare(binVersion, "2.18", ">=") {
		git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "protocol.version=2")
		format += ", Wire Protocol %s Enabled"
		args = append(args, "Version 2") // for focus color
	}

	log.Info(format, args...)
}
