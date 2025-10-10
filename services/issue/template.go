// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/issue/template"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"

	"gopkg.in/yaml.v3"
)

// templateDirCandidates issue templates directory
var templateDirCandidates = []string{
	"ISSUE_TEMPLATE",
	"issue_template",
	".gitea/ISSUE_TEMPLATE",
	".gitea/issue_template",
	".github/ISSUE_TEMPLATE",
	".github/issue_template",
	".gitlab/ISSUE_TEMPLATE",
	".gitlab/issue_template",
}

var templateConfigCandidates = []string{
	".gitea/ISSUE_TEMPLATE/config",
	".gitea/issue_template/config",
	".github/ISSUE_TEMPLATE/config",
	".github/issue_template/config",
}

func GetDefaultTemplateConfig() api.IssueConfig {
	return api.IssueConfig{
		BlankIssuesEnabled: true,
		ContactLinks:       make([]api.IssueConfigContactLink, 0),
	}
}

// GetTemplateConfig loads the given issue config file.
// It never returns a nil config.
func GetTemplateConfig(ctx context.Context, gitRepo *git.Repository, path string, commit *git.Commit) (api.IssueConfig, error) {
	if gitRepo == nil {
		return GetDefaultTemplateConfig(), nil
	}

	treeEntry, err := commit.GetTreeEntryByPath(ctx, path)
	if err != nil {
		return GetDefaultTemplateConfig(), err
	}

	reader, err := treeEntry.Blob().DataAsync(ctx)
	if err != nil {
		log.Debug("DataAsync: %v", err)
		return GetDefaultTemplateConfig(), nil
	}

	defer reader.Close()

	configContent, err := io.ReadAll(reader)
	if err != nil {
		return GetDefaultTemplateConfig(), err
	}

	issueConfig := GetDefaultTemplateConfig()
	if err := yaml.Unmarshal(configContent, &issueConfig); err != nil {
		return GetDefaultTemplateConfig(), err
	}

	for pos, link := range issueConfig.ContactLinks {
		if link.Name == "" {
			return GetDefaultTemplateConfig(), fmt.Errorf("contact_link at position %d is missing name key", pos+1)
		}

		if link.URL == "" {
			return GetDefaultTemplateConfig(), fmt.Errorf("contact_link at position %d is missing url key", pos+1)
		}

		if link.About == "" {
			return GetDefaultTemplateConfig(), fmt.Errorf("contact_link at position %d is missing about key", pos+1)
		}

		_, err = url.ParseRequestURI(link.URL)
		if err != nil {
			return GetDefaultTemplateConfig(), fmt.Errorf("%s is not a valid URL", link.URL)
		}
	}

	return issueConfig, nil
}

// IsTemplateConfig returns if the given path is a issue config file.
func IsTemplateConfig(path string) bool {
	for _, configName := range templateConfigCandidates {
		if path == configName+".yaml" || path == configName+".yml" {
			return true
		}
	}
	return false
}

// ParseTemplatesFromDefaultBranch parses the issue templates in the repo's default branch,
// returns valid templates and the errors of invalid template files (the errors map is guaranteed to be non-nil).
func ParseTemplatesFromDefaultBranch(ctx context.Context, repo *repo.Repository, gitRepo *git.Repository) (ret struct {
	IssueTemplates []*api.IssueTemplate
	TemplateErrors map[string]error
},
) {
	ret.TemplateErrors = map[string]error{}
	if repo.IsEmpty {
		return ret
	}

	commit, err := gitRepo.GetBranchCommit(ctx, repo.DefaultBranch)
	if err != nil {
		return ret
	}

	for _, dirName := range templateDirCandidates {
		tree, err := commit.SubTree(ctx, dirName)
		if err != nil {
			log.Debug("get sub tree of %s: %v", dirName, err)
			continue
		}
		entries, err := tree.ListEntries(ctx)
		if err != nil {
			log.Debug("list entries in %s: %v", dirName, err)
			return ret
		}
		for _, entry := range entries {
			if !template.CouldBe(entry.Name()) {
				continue
			}
			fullName := path.Join(dirName, entry.Name())
			if it, err := template.UnmarshalFromEntry(ctx, entry, dirName); err != nil {
				ret.TemplateErrors[fullName] = err
			} else {
				if !strings.HasPrefix(it.Ref, "refs/") { // Assume that the ref intended is always a branch - for tags users should use refs/tags/<ref>
					it.Ref = git.BranchPrefix + it.Ref
				}
				ret.IssueTemplates = append(ret.IssueTemplates, it)
			}
		}
	}
	return ret
}

// GetTemplateConfigFromDefaultBranch returns the issue config for this repo.
// It never returns a nil config.
func GetTemplateConfigFromDefaultBranch(ctx context.Context, repo *repo.Repository, gitRepo *git.Repository) (api.IssueConfig, error) {
	if repo.IsEmpty {
		return GetDefaultTemplateConfig(), nil
	}

	commit, err := gitRepo.GetBranchCommit(ctx, repo.DefaultBranch)
	if err != nil {
		return GetDefaultTemplateConfig(), err
	}

	for _, configName := range templateConfigCandidates {
		if _, err := commit.GetTreeEntryByPath(ctx, configName+".yaml"); err == nil {
			return GetTemplateConfig(ctx, gitRepo, configName+".yaml", commit)
		}

		if _, err := commit.GetTreeEntryByPath(ctx, configName+".yml"); err == nil {
			return GetTemplateConfig(ctx, gitRepo, configName+".yml", commit)
		}
	}

	return GetDefaultTemplateConfig(), nil
}

func HasTemplatesOrContactLinks(ctx context.Context, repo *repo.Repository, gitRepo *git.Repository) bool {
	ret := ParseTemplatesFromDefaultBranch(ctx, repo, gitRepo)
	if len(ret.IssueTemplates) > 0 {
		return true
	}

	issueConfig, _ := GetTemplateConfigFromDefaultBranch(ctx, repo, gitRepo)
	return len(issueConfig.ContactLinks) > 0
}
