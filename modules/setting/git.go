// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// Git settings
var Git = struct {
	Path                 string
	HomePath             string
	DisableDiffHighlight bool

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

type GitConfigType struct {
	Options map[string]string // git config key is case-insensitive, always use lower-case
}

func (c *GitConfigType) SetOption(key, val string) {
	c.Options[strings.ToLower(key)] = val
}

func (c *GitConfigType) GetOption(key string) string {
	return c.Options[strings.ToLower(key)]
}

var GitConfig = GitConfigType{
	Options: make(map[string]string),
}

func loadGitFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("git")
	if err := sec.MapTo(&Git); err != nil {
		log.Fatal("Failed to map Git settings: %v", err)
	}

	secGitConfig := rootCfg.Section("git.config")
	GitConfig.Options = make(map[string]string)
	GitConfig.SetOption("diff.algorithm", "histogram")
	GitConfig.SetOption("core.logAllRefUpdates", "true")
	GitConfig.SetOption("gc.reflogExpire", "90")

	secGitReflog := rootCfg.Section("git.reflog")
	if secGitReflog.HasKey("ENABLED") {
		deprecatedSetting(rootCfg, "git.reflog", "ENABLED", "git.config", "core.logAllRefUpdates", "1.21")
		GitConfig.SetOption("core.logAllRefUpdates", secGitReflog.Key("ENABLED").In("true", []string{"true", "false"}))
	}
	if secGitReflog.HasKey("EXPIRATION") {
		deprecatedSetting(rootCfg, "git.reflog", "EXPIRATION", "git.config", "core.reflogExpire", "1.21")
		GitConfig.SetOption("gc.reflogExpire", secGitReflog.Key("EXPIRATION").String())
	}

	for _, key := range secGitConfig.Keys() {
		GitConfig.SetOption(key.Name(), key.String())
	}

	Git.HomePath = sec.Key("HOME_PATH").MustString("home")
	if !filepath.IsAbs(Git.HomePath) {
		Git.HomePath = filepath.Join(AppDataPath, Git.HomePath)
	} else {
		Git.HomePath = filepath.Clean(Git.HomePath)
	}
}
