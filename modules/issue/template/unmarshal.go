// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package template

import (
	"fmt"
	"io"
	"path"
	"strconv"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"gopkg.in/yaml.v3"
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
	return unmarshalFromEntry(entry, path.Join(dir, entry.Name())) // Filepaths in Git are ALWAYS '/' separated do not use filepath here
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
		if templateBody, err := markdown.ExtractMetadata(string(content), it); err != nil {
			// The only thing we know here is that we can't extract metadata from the content,
			// it's hard to tell if metadata doesn't exist or metadata isn't valid.
			// There's an example template:
			//
			//    ---
			//    # Title
			//    ---
			//    Content
			//
			// It could be a valid markdown with two horizontal lines, or an invalid markdown with wrong metadata.

			it.Content = string(content)
			it.Name = path.Base(it.FileName) // paths in Git are always '/' separated - do not use filepath!
			it.About = util.EllipsisDisplayString(it.Content, 80)
		} else {
			it.Content = templateBody
			if it.About == "" {
				if _, err := markdown.ExtractMetadata(string(content), compatibleTemplate); err == nil && compatibleTemplate.About != "" {
					it.About = compatibleTemplate.About
				}
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
			// set default id value
			if v.ID == "" {
				v.ID = strconv.Itoa(i)
			}
			// set default visibility
			if v.Visible == nil {
				v.Visible = []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm}
				// markdown is not submitted by default
				if v.Type != api.IssueFormFieldTypeMarkdown {
					v.Visible = append(v.Visible, api.IssueFormFieldVisibleContent)
				}
			}
		}
	}

	return it, nil
}
