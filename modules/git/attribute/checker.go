// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attribute

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/git"
)

// CheckAttributeOpts represents the possible options to CheckAttribute
type CheckAttributeOpts struct {
	CachedOnly    bool
	AllAttributes bool
	Attributes    []string
	Filenames     []string
	IndexFile     string
	WorkTree      string
}

// CheckAttribute return the Blame object of file
func CheckAttribute(repo *git.Repository, opts CheckAttributeOpts) (map[string]Attributes, error) {
	env := []string{}

	if len(opts.IndexFile) > 0 {
		env = append(env, "GIT_INDEX_FILE="+opts.IndexFile)
	}
	if len(opts.WorkTree) > 0 {
		env = append(env, "GIT_WORK_TREE="+opts.WorkTree)
	}

	if len(env) > 0 {
		env = append(os.Environ(), env...)
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmd := git.NewCommand("check-attr", "-z")

	if opts.AllAttributes {
		cmd.AddArguments("-a")
	} else {
		for _, attribute := range opts.Attributes {
			if attribute != "" {
				cmd.AddDynamicArguments(attribute)
			}
		}
	}

	if opts.CachedOnly {
		cmd.AddArguments("--cached")
	}

	cmd.AddDashesAndList(opts.Filenames...)

	if err := cmd.Run(repo.Ctx, &git.RunOpts{
		Env:    env,
		Dir:    repo.Path,
		Stdout: stdOut,
		Stderr: stdErr,
	}); err != nil {
		return nil, fmt.Errorf("failed to run check-attr: %w\n%s\n%s", err, stdOut.String(), stdErr.String())
	}

	// FIXME: This is incorrect on versions < 1.8.5
	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, errors.New("wrong number of fields in return from check-attr")
	}

	name2attribute2info := make(map[string]Attributes)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(Attributes)
		}
		attribute2info[attribute] = Attribute(info)
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, nil
}
