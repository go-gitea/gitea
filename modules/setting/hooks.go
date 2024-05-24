// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

var (
	// Git hook settings
	GitHookPrereceiveName  string
	GitHookPostreceiveName string
	GitHookUpdateName      string
)

func isValidFileName(filename string) error {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("contains path components")
	}
	return nil
}

func loadHooksFrom(rootCfg ConfigProvider) {
	githooks := rootCfg.Section("git.hooks")
	GitHookPrereceiveName = githooks.Key("GIT_HOOK_PRERECEIVE_NAME").MustString("pre-receive")
	GitHookUpdateName = githooks.Key("GIT_HOOK_UPDATE_NAME").MustString("update")
	GitHookPostreceiveName = githooks.Key("GIT_HOOK_POSTRECEIVE_NAME").MustString("post-receive")

	if err := isValidFileName(GitHookPrereceiveName); err != nil {
		log.Fatal("Invalid git pre-receive hook name (%s): %v", GitHookPrereceiveName, err)
	}
	if err := isValidFileName(GitHookUpdateName); err != nil {
		log.Fatal("Invalid git update hook name (%s): %v", GitHookUpdateName, err)
	}
	if err := isValidFileName(GitHookPostreceiveName); err != nil {
		log.Fatal("Invalid git post-receive hook name (%s): %v", GitHookPostreceiveName, err)
	}
}
