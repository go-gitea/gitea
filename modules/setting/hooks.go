// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
)

var (
	// Git hook settings
	GitHookPrereceiveName  string
	GitHookPostreceiveName string
	GitHookUpdateName      string
)

func isValidFileName(filename string) error {
	if filepath.Base(filename) != filename || filepath.IsAbs(filename) || filename == "." || filename == ".." {
		return fmt.Errorf("can only contain filenames, not other directories")
	}
	return nil
}

func loadHooksFrom(rootCfg ConfigProvider) {
	githooks := rootCfg.Section("git.hooks")
	GitHookPrereceiveName = githooks.Key("GIT_HOOK_PRERECEIVE_NAME").MustString("pre-receive")
	GitHookUpdateName = githooks.Key("GIT_HOOK_UPDATE_NAME").MustString("update")
	GitHookPostreceiveName = githooks.Key("GIT_HOOK_POSTRECEIVE_NAME").MustString("post-receive")

	if err := isValidFileName(GitHookPrereceiveName); err != nil {
		log.Fatal("'%s' is an invalid [git.hooks].GIT_HOOK_PRERECEIVE_NAME: %v", GitHookPrereceiveName, err)
	}
	if err := isValidFileName(GitHookUpdateName); err != nil {
		log.Fatal("'%s' is an invalid [git.hooks].GIT_HOOK_UPDATE_NAME: %v", GitHookUpdateName, err)
	}
	if err := isValidFileName(GitHookPostreceiveName); err != nil {
		log.Fatal("'%s' is an invalid [git.hooks].GIT_HOOK_POSTRECEIVE_NAME: %v", GitHookPostreceiveName, err)
	}
}
