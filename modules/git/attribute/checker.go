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
	cancel := func() {}
	envs := []string{"GIT_FLUSH=1"}
	cmd := git.NewCommand("check-attr", "-z")
	if len(attributes) == 0 {
		cmd.AddArguments("--all")
	}

	// there is treeish, read from bare repo or temp index created by "read-tree"
	if treeish != "" {
		if git.DefaultFeatures().SupportCheckAttrOnBare {
			cmd.AddArguments("--source")
			cmd.AddDynamicArguments(treeish)
		} else {
			indexFilename, worktree, deleteTemporaryFile, err := gitRepo.ReadTreeToTemporaryIndex(treeish)
			if err != nil {
				return nil, nil, nil, err
			}

			cmd.AddArguments("--cached")
			envs = append(envs,
				"GIT_INDEX_FILE="+indexFilename,
				"GIT_WORK_TREE="+worktree,
			)
			cancel = deleteTemporaryFile
		}
	} else {
		// Read from existing index, in cases where the repo is bare and has an index,
		// or the work tree contains unstaged changes that shouldn't affect the attribute check.
		// It is caller's responsibility to add changed ".gitattributes" into the index if they want to respect the new changes.
		cmd.AddArguments("--cached")
	}

	cmd.AddDynamicArguments(attributes...)
	if len(filenames) > 0 {
		cmd.AddDashesAndList(filenames...)
	}
	return cmd, envs, cancel, nil
}

type CheckAttributeOpts struct {
	Filenames  []string
	Attributes []string
}

// CheckAttributes return the attributes of the given filenames and attributes in the given treeish.
// If treeish is empty, then it will use current working directory, otherwise it will use the provided treeish on the bare repo
func CheckAttributes(ctx context.Context, gitRepo *git.Repository, treeish string, opts CheckAttributeOpts) (map[string]*Attributes, error) {
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

	attributesMap := make(map[string]*Attributes)
	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info, ok := attributesMap[filename]
		if !ok {
			attribute2info = NewAttributes()
			attributesMap[filename] = attribute2info
		}
		attribute2info.m[attribute] = Attribute(info)
	}

	return attributesMap, nil
}
