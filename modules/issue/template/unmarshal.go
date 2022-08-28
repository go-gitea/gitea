// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"io"
	"strconv"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"gopkg.in/yaml.v2"
)

// TODO
func Unmarshal(filename string, content []byte) (*api.IssueTemplate, error) {
	it := &api.IssueTemplate{
		FileName: filename,
	}

	if typ := it.Type(); typ == "md" {
		templateBody, err := markdown.ExtractMetadata(string(content), it)
		if err != nil {
			return nil, fmt.Errorf("extract metadata: %w", err)
		}
		it.Content = templateBody
	} else if typ == "yaml" {
		if err := yaml.Unmarshal(content, it); err != nil {
			return nil, fmt.Errorf("yaml unmarshal: %w", err)
		}
		if it.About == "" {
			// Compatible with treating description as about
			compatibleTemplate := &struct {
				About string `yaml:"description"`
			}{}
			if err := yaml.Unmarshal(content, compatibleTemplate); err == nil && compatibleTemplate.About != "" {
				it.About = compatibleTemplate.About
			}
		}
		for i, v := range it.Fields {
			if v.ID == "" {
				v.ID = strconv.Itoa(i)
			}
		}
	}

	return it, nil
}

// TODO
func UnmarshalFromEntry(entry *git.TreeEntry) (*api.IssueTemplate, error) {
	if size := entry.Blob().Size(); size > setting.UI.MaxDisplayFileSize {
		return nil, fmt.Errorf("too large: %v > MaxDisplayFileSize", size)
	}

	r, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, fmt.Errorf("data async: %w", err)
	}
	defer r.Close()

	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read all: %w", err)
	}

	return Unmarshal(entry.Name(), content)
}

// TODO
func UnmarshalFromCommit(commit *git.Commit, filename string) (*api.IssueTemplate, error) {
	entry, err := commit.GetTreeEntryByPath(filename)
	if err != nil {
		return nil, fmt.Errorf("get entry for %q: %w", filename, err)
	}
	return UnmarshalFromEntry(entry)
}

// TODO
func UnmarshalFromRepo(repo *git.Repository, branch, filename string) (*api.IssueTemplate, error) {
	commit, err := repo.GetBranchCommit(branch)
	if err != nil {
		return nil, fmt.Errorf("get commit on branch %q: %w", branch, err)
	}

	return UnmarshalFromCommit(commit, filename)
}
