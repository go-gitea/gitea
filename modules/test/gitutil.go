// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"bytes"
	"fmt"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
)

type GitFastImportFile struct {
	Mode    git.EntryMode
	Path    string
	Content string
}

type GitFastImportCommit struct {
	Ref     string
	Message string
	Files   []GitFastImportFile
}

func GitFastImport(t TestingT, repo git.RepositoryFacade, commits []GitFastImportCommit) {
	var buf bytes.Buffer
	for i, c := range commits {
		_, _ = fmt.Fprintf(&buf, "reset %s\n", c.Ref)
		_, _ = fmt.Fprintf(&buf, "commit %s\nmark :%d\ncommitter Gitea <gitea@example.com> 1500000000 +0000\n", c.Ref, i+1)
		_, _ = fmt.Fprintf(&buf, "data %d\n%s\n", len(c.Message), c.Message)
		for _, f := range c.Files {
			_, _ = fmt.Fprintf(&buf, "M %s inline %s\ndata %d\n%s\n", f.Mode.String(), f.Path, len(f.Content), f.Content)
		}
	}
	buf.WriteString("done\n")
	_, _, err := gitcmd.NewCommand("fast-import").AddArguments("--force", "--done").
		WithRepo(repo).WithStdinBytes(buf.Bytes()).
		RunStdString(t.Context())
	if err != nil {
		t.Fatalf("Failed to do git fast-import for repo: %v", err)
	}
}
