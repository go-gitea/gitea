// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// Git settings
var Git = struct {
	Path                      string
	HomePath                  string
	DisableDiffHighlight      bool
	MaxGitDiffLines           int
	MaxGitDiffLineCharacters  int
	MaxGitDiffFiles           int
	CommitsRangeSize          int // CommitsRangeSize the default commits range size
	BranchesRangeSize         int // BranchesRangeSize the default branches range size
	VerbosePush               bool
	VerbosePushDelay          time.Duration
	GCArgs                    []string `ini:"GC_ARGS" delim:" "`
	EnableAutoGitWireProtocol bool
	PullRequestPushMessage    bool
	LargeObjectThreshold      int64
	DisableCoreProtectNTFS    bool
	DisablePartialClone       bool
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
	CommitsRangeSize:          50,
	BranchesRangeSize:         20,
	VerbosePush:               true,
	VerbosePushDelay:          5 * time.Second,
	GCArgs:                    []string{},
	EnableAutoGitWireProtocol: true,
	PullRequestPushMessage:    true,
	LargeObjectThreshold:      1024 * 1024,
	DisablePartialClone:       false,
	Timeout: struct {
		Default int
		Migrate int
		Mirror  int
		Clone   int
		Pull    int
		GC      int `ini:"GC"`
	}{
		Default: 360,
		Migrate: 600,
		Mirror:  300,
		Clone:   300,
		Pull:    300,
		GC:      60,
	},
}

func newGit() {
	sec := Cfg.Section("git")

	if err := sec.MapTo(&Git); err != nil {
		log.Fatal("Failed to map Git settings: %v", err)
	}

	Git.HomePath = sec.Key("HOME_PATH").MustString("home")
	if !filepath.IsAbs(Git.HomePath) {
		Git.HomePath = filepath.Join(AppDataPath, Git.HomePath)
	} else {
		Git.HomePath = filepath.Clean(Git.HomePath)
	}
}
