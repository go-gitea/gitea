// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "gitea.dev/modules/git/gitcmd"

func HandleGitCmdHTTPRedirection(cmd *gitcmd.Command, targets ...string) {
	// Protect from SSRF vector (e.g. migrating from an attacker URL).
	// cmd.AddConfig("http.followRedirects", "false")
	// However, we can't do so at the moment:
	// this fails due to 301: git -c http.followRedirects=false clone -v https://gitlab.com/{owner}/{repo}
	// this succeeds: git -c http.followRedirects=false clone -v https://gitlab.com/{owner}/{repo}.git
	// FIXME: GIT-CLONE-HTTP-REDIRECT-SSRF: need a complete solution in the future
}
