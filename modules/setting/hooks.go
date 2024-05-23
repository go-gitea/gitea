// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

var (
	// Git hook settings
	GitHookPrereceiveName  string
	GitHookPostreceiveName string
	GitHookUpdateName      string
)

func loadHooksFrom(rootCfg ConfigProvider) {
	githooks := rootCfg.Section("GitHooks")
	GitHookPrereceiveName = githooks.Key("GIT_HOOK_PRERECEIVE_NAME").MustString("pre-receive")
	GitHookUpdateName = githooks.Key("GIT_HOOK_UPDATE_NAME").MustString("update")
	GitHookPostreceiveName = githooks.Key("GIT_HOOK_POSTRECEIVE_NAME").MustString("post-receive")
}
