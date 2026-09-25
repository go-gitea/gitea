// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"fmt"
	"os/exec"
	"sync/atomic"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

var GitExecutable = "git" // the command name of git, will be updated to an absolute path during initialization

var httpProxyEnvs atomic.Pointer[[]string]

// SetHTTPProxy routes the http(s) remotes of every git command through proxyURL, "" disables it.
func SetHTTPProxy(proxyURL string) {
	var envs []string
	if proxyURL != "" {
		// command scope config beats every config file and keeps the credentials out of process listings, git also honors no_proxy for a configured proxy
		envs = []string{"GIT_CONFIG_PARAMETERS='http.proxy=" + proxyURL + "' 'http.proxyAuthMethod=basic'", "no_proxy=", "NO_PROXY="}
	}
	httpProxyEnvs.Store(&envs)
}

// SetExecutablePath changes the path of git executable and checks the file permission and version.
func SetExecutablePath(path string) error {
	// If path is empty, we use the default value of GitExecutable "git" to search for the location of git.
	if path != "" {
		GitExecutable = path
	}
	absPath, err := exec.LookPath(GitExecutable)
	if err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	GitExecutable = absPath
	return nil
}

// HomeDir is the home dir for git to store the global config file used by Gitea internally
func HomeDir() string {
	if setting.Git.HomePath == "" {
		// strict check, make sure the git module is initialized correctly.
		// attention: when the git module is called in gitea sub-command (serv/hook), the log module might not obviously show messages to users/developers.
		// for example: if there is gitea git hook code calling NewCommand before git.InitXxx, the integration test won't show the real failure reasons.
		log.Fatal("Unable to init Git's HomeDir, incorrect initialization of the setting and git modules")
		return ""
	}
	return setting.Git.HomePath
}
