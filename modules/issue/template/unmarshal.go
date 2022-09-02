// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"gopkg.in/yaml.v2"
)

// CouldBe indicates a file with the filename could be a template,
// it is a low cost check before further processing.
func CouldBe(filename string) bool {
	it := &api.IssueTemplate{
		FileName: filename,
	}
	return it.Type() != ""
}

// Unmarshal parses out a valid template from the content
func Unmarshal(filename string, content []byte) (*api.IssueTemplate, error) {
	it, err := unmarshal(filename, content)
	if err != nil {
		return nil, err
	}

	if err := Validate(it); err != nil {
		return nil, err
	}

	return it, nil
}

// UnmarshalFromEntry parses out a valid template from the blob in entry
func UnmarshalFromEntry(entry *git.TreeEntry, dir string) (*api.IssueTemplate, error) {
	return unmarshalFromEntry(entry, filepath.Join(dir, entry.Name()))
}

// UnmarshalFromCommit parses out a valid template from the commit
func UnmarshalFromCommit(commit *git.Commit, filename string) (*api.IssueTemplate, error) {
	entry, err := commit.GetTreeEntryByPath(filename)
	if err != nil {
		return nil, fmt.Errorf("get entry for %q: %w", filename, err)
	}
	return unmarshalFromEntry(entry, filename)
}

// UnmarshalFromRepo parses out a valid template from the head commit of the branch
func UnmarshalFromRepo(repo *git.Repository, branch, filename string) (*api.IssueTemplate, error) {
	commit, err := repo.GetBranchCommit(branch)
	if err != nil {
		return nil, fmt.Errorf("get commit on branch %q: %w", branch, err)
	}

	return UnmarshalFromCommit(commit, filename)
}

func unmarshalFromEntry(entry *git.TreeEntry, filename string) (*api.IssueTemplate, error) {
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

	return Unmarshal(filename, content)
}

func unmarshal(filename string, content []byte) (*api.IssueTemplate, error) {
	it := &api.IssueTemplate{
		FileName: filename,
	}

	// Compatible with treating description as about
	compatibleTemplate := &struct {
		About string `yaml:"description"`
	}{}

	if typ := it.Type(); typ == api.IssueTemplateTypeMarkdown {
		templateBody, err := markdown.ExtractMetadata(string(content), it)
		if err != nil {
			return nil, err
		}
		it.Content = templateBody
		if it.About == "" {
			if _, err := markdown.ExtractMetadata(string(content), compatibleTemplate); err == nil && compatibleTemplate.About != "" {
				it.About = compatibleTemplate.About
			}
		}
	} else if typ == api.IssueTemplateTypeYaml {
		if err := yaml.Unmarshal(content, it); err != nil {
			return nil, fmt.Errorf("yaml unmarshal: %w", err)
		}
		if it.About == "" {
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
