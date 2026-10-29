// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/issue/template"
	"gitea.dev/modules/log"
	"gitea.dev/modules/reqctx"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/ownerrepo"

	"go.yaml.in/yaml/v4"
)

var IssueTemplateCandidates = []string{
	"ISSUE_TEMPLATE.md",
	"ISSUE_TEMPLATE.yaml",
	"ISSUE_TEMPLATE.yml",
	"issue_template.md",
	"issue_template.yaml",
	"issue_template.yml",
	".gitea/ISSUE_TEMPLATE.md",
	".gitea/ISSUE_TEMPLATE.yaml",
	".gitea/ISSUE_TEMPLATE.yml",
	".gitea/issue_template.md",
	".gitea/issue_template.yaml",
	".gitea/issue_template.yml",
	".github/ISSUE_TEMPLATE.md",
	".github/ISSUE_TEMPLATE.yaml",
	".github/ISSUE_TEMPLATE.yml",
	".github/issue_template.md",
	".github/issue_template.yaml",
	".github/issue_template.yml",
}

var PullRequestTemplateCandidates = []string{
	"PULL_REQUEST_TEMPLATE.md",
	"PULL_REQUEST_TEMPLATE.yaml",
	"PULL_REQUEST_TEMPLATE.yml",
	"pull_request_template.md",
	"pull_request_template.yaml",
	"pull_request_template.yml",
	".gitea/PULL_REQUEST_TEMPLATE.md",
	".gitea/PULL_REQUEST_TEMPLATE.yaml",
	".gitea/PULL_REQUEST_TEMPLATE.yml",
	".gitea/pull_request_template.md",
	".gitea/pull_request_template.yaml",
	".gitea/pull_request_template.yml",
	".github/PULL_REQUEST_TEMPLATE.md",
	".github/PULL_REQUEST_TEMPLATE.yaml",
	".github/PULL_REQUEST_TEMPLATE.yml",
	".github/pull_request_template.md",
	".github/pull_request_template.yaml",
	".github/pull_request_template.yml",
}

var issueTemplateSourcePaths = slices.Concat(templateDirCandidates, IssueTemplateCandidates)

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

	treeEntry, err := commit.GetTreeEntryByPath(ctx, gitRepo, path)
	if err != nil {
		return GetDefaultTemplateConfig(), err
	}

	reader, err := treeEntry.Blob(gitRepo).DataAsync(ctx)
	if err != nil {
		log.Debug("DataAsync: %v", err)
		return GetDefaultTemplateConfig(), nil
	}

	defer reader.Close()

	configContent, err := util.ReadWithLimit(reader, 1024*1024)
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
		tree, err := commit.SubTree(ctx, gitRepo, dirName)
		if err != nil {
			log.Debug("get sub tree of %s: %v", dirName, err)
			continue
		}
		entries, err := tree.ListEntries(ctx, gitRepo)
		if err != nil {
			log.Debug("list entries in %s: %v", dirName, err)
			return ret
		}
		for _, entry := range entries {
			if !template.CouldBe(entry.Name()) {
				continue
			}
			fullName := path.Join(dirName, entry.Name())
			if it, err := template.UnmarshalFromEntry(ctx, gitRepo, entry, dirName); err != nil {
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
		if _, err := commit.GetTreeEntryByPath(ctx, gitRepo, configName+".yaml"); err == nil {
			return GetTemplateConfig(ctx, gitRepo, configName+".yaml", commit)
		}

		if _, err := commit.GetTreeEntryByPath(ctx, gitRepo, configName+".yml"); err == nil {
			return GetTemplateConfig(ctx, gitRepo, configName+".yml", commit)
		}
	}

	return GetDefaultTemplateConfig(), nil
}

func GetIssueTemplateSourceRepo(ctx reqctx.RequestContext, doer *user_model.User, targetRepo *repo.Repository, targetGitRepo *git.Repository, templatePath string) (*repo.Repository, *git.Repository) {
	return getTemplateSourceRepo(ctx, doer, targetRepo, targetGitRepo, templatePath, issueTemplateSourcePaths)
}

func GetPullRequestTemplateSourceRepo(ctx reqctx.RequestContext, doer *user_model.User, targetRepo *repo.Repository, targetGitRepo *git.Repository, templatePath string) (*repo.Repository, *git.Repository) {
	return getTemplateSourceRepo(ctx, doer, targetRepo, targetGitRepo, templatePath, PullRequestTemplateCandidates)
}

func getTemplateSourceRepo(ctx reqctx.RequestContext, doer *user_model.User, targetRepo *repo.Repository, targetGitRepo *git.Repository, templatePath string, paths []string) (*repo.Repository, *git.Repository) {
	if !targetRepo.IsEmpty {
		if commit, err := targetGitRepo.GetBranchCommit(ctx, targetRepo.DefaultBranch); err == nil && hasAnyPath(ctx, targetGitRepo, commit, templatePath, paths) {
			return targetRepo, targetGitRepo
		}
	}
	found := ownerrepo.Find(ctx, doer, targetRepo.Owner, ownerrepo.DefaultsRepoNames, func(found *ownerrepo.Repo) bool {
		return hasAnyPath(ctx, found.GitRepo, found.Commit, "", paths)
	})
	if found == nil {
		return targetRepo, targetGitRepo
	}
	return found.Repo, found.GitRepo
}

func hasAnyPath(ctx context.Context, gitRepo *git.Repository, commit *git.Commit, templatePath string, paths []string) bool {
	trees := map[string]*git.Tree{} // one tree read per directory instead of per path
	exists := func(treePath string) bool {
		dir, name := path.Split(path.Clean(treePath))
		tree, ok := trees[dir]
		if !ok {
			tree, _ = commit.SubTree(ctx, gitRepo, strings.TrimSuffix(dir, "/"))
			trees[dir] = tree
		}
		if tree == nil {
			return false
		}
		_, err := tree.GetTreeEntryByPath(ctx, gitRepo, name)
		return err == nil
	}
	return templatePath != "" && exists(templatePath) || slices.ContainsFunc(paths, exists)
}
