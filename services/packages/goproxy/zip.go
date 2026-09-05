// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"context"
	"io"
	"path/filepath"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"

	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

// CreateZip writes a valid Go module zip for the version to w.
func (v *Version) CreateZip(ctx context.Context, w io.Writer) error {
	if _, err := v.GoMod(ctx); err != nil {
		return err
	}

	tmpDir, cleanup, err := setting.AppDataTempDir("goproxy").MkdirTempRandom("module")
	if err != nil {
		return err
	}
	defer cleanup()

	repoPath := gitrepo.RepoLocalPath(v.RepoFacade)
	if err := git.Clone(ctx, repoPath, tmpDir, git.CloneRepoOptions{Shared: true, NoCheckout: true}); err != nil {
		return err
	}

	if _, _, err := gitcmd.NewCommand("checkout", "--detach").AddDynamicArguments(v.CommitID).WithDir(tmpDir).RunStdString(ctx); err != nil {
		return err
	}

	dir := tmpDir
	if v.Subdir != "" {
		dir = filepath.Join(tmpDir, filepath.FromSlash(v.Subdir))
	}

	return zip.CreateFromDir(w, module.Version{Path: v.ModulePath, Version: v.Version}, dir)
}
