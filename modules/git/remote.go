// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"

	giturl "code.gitea.io/gitea/modules/git/url"
)

// GetRemoteAddress returns remote url of git repository in the repoPath with special remote name
func GetRemoteAddress(ctx context.Context, repoPath, remoteName string) (string, error) {
	var cmd *Command
	if DefaultFeatures().CheckVersionAtLeast("2.7") {
		cmd = NewCommand(ctx, "remote", "get-url").AddDynamicArguments(remoteName)
	} else {
		cmd = NewCommand(ctx, "config", "--get").AddDynamicArguments("remote." + remoteName + ".url")
	}

	result, _, err := cmd.RunStdString(&RunOpts{Dir: repoPath})
	if err != nil {
		return "", err
	}

	if len(result) > 0 {
		result = result[:len(result)-1]
	}
	return result, nil
}

// GetRemoteURL returns the url of a specific remote of the repository.
func GetRemoteURL(ctx context.Context, repoPath, remoteName string) (*giturl.GitURL, error) {
	addr, err := GetRemoteAddress(ctx, repoPath, remoteName)
	if err != nil {
		return nil, err
	}
	return giturl.Parse(addr)
}

// IsRemoteNotExistError checks the prefix of the error message to see whether a remote does not exist.
func IsRemoteNotExistError(err error) bool {
	// see: https://github.com/go-gitea/gitea/issues/32889#issuecomment-2571848216
	// Should not add space in the end, sometimes git will add a `:`
	prefix1 := "exit status 128 - fatal: No such remote" // git < 2.30
	prefix2 := "exit status 2 - error: No such remote"   // git >= 2.30
	return strings.HasPrefix(err.Error(), prefix1) || strings.HasPrefix(err.Error(), prefix2)
}
