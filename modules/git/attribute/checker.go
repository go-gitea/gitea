// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attribute

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/git"
)

func checkAttrCommand(gitRepo *git.Repository, treeish string, filenames, attributes []string) (*git.Command, []string, func(), error) {
	cmd := git.NewCommand("check-attr", "-z")
	if len(attributes) == 0 {
		cmd.AddArguments("--all")
	}
	cmd.AddDashesAndList(filenames...)
	if git.DefaultFeatures().SupportCheckAttrOnBare && treeish != "" {
		cmd.AddArguments("--source")
		cmd.AddDynamicArguments(treeish)
		cmd.AddDynamicArguments(attributes...)
		return cmd, []string{"GIT_FLUSH=1"}, nil, nil
	}

	var cancel func()
	var envs []string
	if treeish != "" { // if it's empty, then we assume it's a worktree repository
		indexFilename, worktree, deleteTemporaryFile, err := gitRepo.ReadTreeToTemporaryIndex(treeish)
		if err != nil {
			return nil, nil, nil, err
		}

		envs = []string{
			"GIT_INDEX_FILE=" + indexFilename,
			"GIT_WORK_TREE=" + worktree,
			"GIT_FLUSH=1",
		}
		cancel = deleteTemporaryFile
	}
	cmd.AddArguments("--cached")
	cmd.AddDynamicArguments(attributes...)
	return cmd, envs, cancel, nil
}

type CheckAttributeOpts struct {
	Filenames  []string
	Attributes []string
}

// CheckAttribute return the Blame object of file
func CheckAttribute(ctx context.Context, gitRepo *git.Repository, treeish string, opts CheckAttributeOpts) (map[string]Attributes, error) {
	cmd, envs, cancel, err := checkAttrCommand(gitRepo, treeish, opts.Filenames, opts.Attributes)
	if err != nil {
		return nil, err
	}
	defer cancel()

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	if err := cmd.Run(ctx, &git.RunOpts{
		Env:    append(os.Environ(), envs...),
		Dir:    gitRepo.Path,
		Stdout: stdOut,
		Stderr: stdErr,
	}); err != nil {
		return nil, fmt.Errorf("failed to run check-attr: %w\n%s\n%s", err, stdOut.String(), stdErr.String())
	}

	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, errors.New("wrong number of fields in return from check-attr")
	}

	attributesMap := make(map[string]Attributes)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := attributesMap[filename]
		if attribute2info == nil {
			attribute2info = make(Attributes)
		}
		attribute2info[attribute] = Attribute(info)
		attributesMap[filename] = attribute2info
	}

	return attributesMap, nil
}
