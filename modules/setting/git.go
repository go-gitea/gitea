// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/git"
	gitsetting "code.gitea.io/gitea/modules/git/setting"
	"code.gitea.io/gitea/modules/log"
)

var (
	// Git settings
	Git = gitsetting.Config{
		DisableDiffHighlight:      false,
		MaxGitDiffLines:           1000,
		MaxGitDiffLineCharacters:  5000,
		MaxGitDiffFiles:           100,
		VerbosePush:               true,
		VerbosePushDelay:          5 * time.Second,
		GCArgs:                    []string{},
		EnableAutoGitWireProtocol: true,
		PullRequestPushMessage:    true,
		Timeout: gitsetting.TimeoutConfig{
			Default: int(git.DefaultCommandExecutionTimeout / time.Second),
			Migrate: 600,
			Mirror:  300,
			Clone:   300,
			Pull:    300,
			GC:      60,
		},
		ProviderType: "native",
	}
)

func newGit() {
	if err := Cfg.Section("git").MapTo(&Git); err != nil {
		log.Fatal("Failed to map Git settings: %v", err)
	}
	gitsetting.NewGitService(Git)
}
