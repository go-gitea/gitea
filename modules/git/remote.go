// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"net/url"
)

// GetRemoteAddress returns the url of a specific remote of the repository.
func GetRemoteAddress(ctx context.Context, repoPath, remoteName string) (*url.URL, error) {
	err := LoadGitVersion()
	if err != nil {
		return nil, err
	}
	var cmd *Command
	if CheckGitVersionAtLeast("2.7") == nil {
		cmd = NewCommand(ctx, "remote", "get-url", remoteName)
	} else {
		cmd = NewCommand(ctx, "config", "--get", "remote."+remoteName+".url")
	}

	result, err := cmd.RunInDir(repoPath)
	if err != nil {
		return nil, err
	}

	if len(result) > 0 {
		result = result[:len(result)-1]
	}
	return url.Parse(result)
}
