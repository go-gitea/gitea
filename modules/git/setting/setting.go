// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

// Setting handles configuring the git module

import (
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	// force providers to auto-register
	_ "code.gitea.io/gitea/modules/git/providers/gogit"
	// force providers to auto-register
	_ "code.gitea.io/gitea/modules/git/providers/native"
)

// Config represents the git configuration
type Config struct {
	Path                      string
	DisableDiffHighlight      bool
	MaxGitDiffLines           int
	MaxGitDiffLineCharacters  int
	MaxGitDiffFiles           int
	VerbosePush               bool
	VerbosePushDelay          time.Duration
	GCArgs                    []string `ini:"GC_ARGS" delim:" "`
	EnableAutoGitWireProtocol bool
	PullRequestPushMessage    bool
	Timeout                   TimeoutConfig `ini:"git.timeout"`
	ProviderType              string
}

// TimeoutConfig represents the timeout configuration
type TimeoutConfig struct {
	Default int
	Migrate int
	Mirror  int
	Clone   int
	Pull    int
	GC      int `ini:"GC"`
}

// NewGitService configures the git service
func NewGitService(config Config) {
	if err := git.SetExecutablePath(config.Path); err != nil {
		log.Fatal("Failed to initialize Git settings: %v", err)
	}
	git.DefaultCommandExecutionTimeout = time.Duration(config.Timeout.Default) * time.Second

	version, err := git.LocalVersion()
	if err != nil {
		log.Fatal("Error retrieving git version: %v", err)
	}

	// force cleanup args
	git.GlobalCommandArgs = []string{}

	if git.CheckGitVersionAtLeast("2.9") == nil {
		// Explicitly disable credential helper, otherwise Git credentials might leak
		git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "credential.helper=")
	}

	var format = "Git Version: %s"
	var args = []interface{}{version.Original()}
	// Since git wire protocol has been released from git v2.18
	if config.EnableAutoGitWireProtocol && git.CheckGitVersionAtLeast("2.18") == nil {
		git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "protocol.version=2")
		format += ", Wire Protocol %s Enabled"
		args = append(args, "Version 2") // for focus color
	}

	git.SetServiceProvider(config.ProviderType)

	log.Info(format, args...)
}
