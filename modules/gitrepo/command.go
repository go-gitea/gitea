// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

type CmdOption struct {
	Env []string
}

func WithEnv(env []string) func(opt *CmdOption) {
	return func(opt *CmdOption) {
		opt.Env = env
	}
}

func RunCmdString(ctx context.Context, repo Repository, cmd *gitcmd.Command, opts ...func(opt *CmdOption)) (string, error) {
	var opt CmdOption
	for _, o := range opts {
		o(&opt)
	}
	res, _, err := cmd.RunStdString(ctx, &gitcmd.RunOpts{
		Dir: repoPath(repo),
		Env: opt.Env,
	})
	return res, err
}
