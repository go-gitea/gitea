// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// AttributeService provides a mechanism to check an attribute
type AttributeService struct {
}

// CheckAttribute returns an attribute map
func (AttributeService) CheckAttribute(repo service.Repository, opts service.CheckAttributeOpts) (map[string]map[string]string, error) {
	err := git.LoadGitVersion()
	if err != nil {
		return nil, fmt.Errorf("Git version missing: %v", err)
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmdArgs := []string{"check-attr", "-z"}

	if opts.AllAttributes {
		cmdArgs = append(cmdArgs, "-a")
	} else {
		for _, attribute := range opts.Attributes {
			if attribute != "" {
				cmdArgs = append(cmdArgs, attribute)
			}
		}
	}

	// git check-attr --cached first appears in git 1.7.8
	if opts.CachedOnly && git.CheckGitVersionAtLeast("1.7.8") == nil {
		cmdArgs = append(cmdArgs, "--cached")
	}

	cmdArgs = append(cmdArgs, "--")

	for _, arg := range opts.Filenames {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := git.NewCommand(cmdArgs...)

	if err := cmd.RunInDirPipeline(repo.Path(), stdOut, stdErr); err != nil {
		return nil, fmt.Errorf("Failed to run check-attr: %v\n%s\n%s", err, stdOut.String(), stdErr.String())
	}

	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, fmt.Errorf("Wrong number of fields in return from check-attr")
	}

	var name2attribute2info = make(map[string]map[string]string)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(map[string]string)
		}
		attribute2info[attribute] = info
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, nil
}
