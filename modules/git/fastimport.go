// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/util"
)

type FastImportInit struct {
	Bare         bool
	ObjectFormat string
}

type FastImportFile struct {
	Mode    EntryMode
	Path    string
	Content string
}

type FastImportCommit struct {
	Ref     string
	Message string
	Files   []FastImportFile
}

// ForceFastImportWithInit is for mainly for testing purpose
func ForceFastImportWithInit(ctx context.Context, repoLocalPath string, commits []FastImportCommit, initOpts ...FastImportInit) (RepositoryFacade, error) {
	repo := gitrepo.RepositoryUnmanaged(repoLocalPath)
	initOpt := util.OptionalArg(initOpts, FastImportInit{Bare: true})
	if exist, _ := IsRepositoryExist(ctx, repo); !exist {
		_ = os.MkdirAll(repoLocalPath, 0o755)
		err := InitRepositoryLocal(ctx, repoLocalPath, initOpt.Bare, util.IfZero(initOpt.ObjectFormat, "sha1"))
		if err != nil {
			return nil, err
		}
	}
	return repo, ForceFastImport(ctx, repo, commits)
}

// ForceFastImport is for mainly for testing purpose
func ForceFastImport(ctx context.Context, repo RepositoryFacade, commits []FastImportCommit) error {
	var buf bytes.Buffer
	for i, c := range commits {
		msg := util.IfZero(c.Message, fmt.Sprintf("commit %d", i+1))
		_, _ = fmt.Fprintf(&buf, "reset %s\n", c.Ref)
		_, _ = fmt.Fprintf(&buf, "commit %s\nmark :%d\ncommitter Gitea <gitea@example.com> 1500000000 +0000\n", c.Ref, i+1)
		_, _ = fmt.Fprintf(&buf, "data %d\n%s\n", len(msg), msg)
		for _, f := range c.Files {
			mode := util.IfZero(f.Mode, EntryModeBlob)
			_, _ = fmt.Fprintf(&buf, "M %s inline %s\ndata %d\n%s\n", mode.String(), f.Path, len(f.Content), f.Content)
		}
	}
	buf.WriteString("done\n")
	_, _, err := gitcmd.NewCommand("fast-import").AddArguments("--force", "--done").
		WithRepo(repo).WithStdinBytes(buf.Bytes()).
		RunStdString(ctx)
	return err
}
