// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/log"
)

type TemplateSubmoduleCommit struct {
	Path   string
	Commit string
}

// GetTemplateSubmoduleCommits returns a list of submodules paths and their commits from a repository
// This function is only for generating new repos based on existing template, the template couldn't be too large.
func GetTemplateSubmoduleCommits(ctx context.Context, repoPath string) (submoduleCommits []TemplateSubmoduleCommit, _ error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	opts := &RunOpts{
		Dir:    repoPath,
		Stdout: stdoutWriter,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			defer stdoutReader.Close()

			scanner := bufio.NewScanner(stdoutReader)
			for scanner.Scan() {
				entry, err := parseLsTreeLine(scanner.Bytes())
				if err != nil {
					cancel()
					return err
				}
				if entry.EntryMode == EntryModeCommit {
					submoduleCommits = append(submoduleCommits, TemplateSubmoduleCommit{Path: entry.Name, Commit: entry.ID.String()})
				}
			}
			return scanner.Err()
		},
	}
	err = NewCommand("ls-tree", "-r", "--", "HEAD").Run(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("GetTemplateSubmoduleCommits: error running git ls-tree: %v", err)
	}
	return submoduleCommits, nil
}

// AddTemplateSubmoduleIndexes Adds the given submodules to the git index.
// It is only for generating new repos based on existing template, requires the .gitmodules file to be already present in the work dir.
func AddTemplateSubmoduleIndexes(ctx context.Context, repoPath string, submodules []TemplateSubmoduleCommit) error {
	for _, submodule := range submodules {
		cmd := NewCommand("update-index", "--add", "--cacheinfo", "160000").AddDynamicArguments(submodule.Commit, submodule.Path)
		if stdout, _, err := cmd.RunStdString(ctx, &RunOpts{Dir: repoPath}); err != nil {
			log.Error("Unable to add %s as submodule to repo %s: stdout %s\nError: %v", submodule.Path, repoPath, stdout, err)
			return err
		}
	}
	return nil
}
