// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"io"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/util"
)

var OpenRepository = git.OpenRepository // TODO: can be removed in the future

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	key string
}

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
// The caller must call Closer.Close()
func RepositoryFromContextOrOpen(ctx context.Context, repo git.RepositoryFacade) (*git.Repository, io.Closer, error) {
	reqCtx := reqctx.FromContext(ctx)
	if reqCtx != nil {
		gitRepo, err := RepositoryFromRequestContextOrOpen(reqCtx, repo)
		return gitRepo, util.NopCloser{}, err
	}
	gitRepo, err := OpenRepository(repo)
	return gitRepo, gitRepo, err
}

// RepositoryFromRequestContextOrOpen opens the repository at the given relative path in the provided request context.
// Caller shouldn't close the git repo manually, the git repo will be automatically closed when the request context is done.
func RepositoryFromRequestContextOrOpen(ctx reqctx.RequestContext, repo git.RepositoryFacade) (*git.Repository, error) {
	ck := contextKey{key: repo.GitRepoLocation()}
	if gitRepo, ok := ctx.Value(ck).(*git.Repository); ok {
		return gitRepo, nil
	}
	gitRepo, err := git.OpenRepository(repo)
	if err != nil {
		return nil, err
	}
	ctx.AddCloser(gitRepo)
	ctx.SetContextValue(ck, gitRepo)
	return gitRepo, nil
}

func UpdateServerInfo(ctx context.Context, repo git.RepositoryFacade) error {
	_, _, err := gitcmd.NewCommand("update-server-info").WithRepo(repo).RunStdBytes(ctx)
	return err
}
