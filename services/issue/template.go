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

	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/issue/template"
	"gitea.dev/modules/log"
	"gitea.dev/modules/reqctx"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"

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

func TemplateRepo(ctx reqctx.RequestContext, doer *user_model.User, targetRepo *repo.Repository, targetGitRepo *git.Repository, templatePath string) (*repo.Repository, *git.Repository) {
	return TemplateRepoForPaths(ctx, doer, targetRepo, targetGitRepo, templatePath, slices.Concat(templateDirCandidates, IssueTemplateCandidates))
}

func TemplateRepoForPaths(ctx reqctx.RequestContext, doer *user_model.User, targetRepo *repo.Repository, targetGitRepo *git.Repository, templatePath string, paths []string) (*repo.Repository, *git.Repository) {
	if hasAnyPath(ctx, targetRepo, targetGitRepo, templatePath, paths) {
		return targetRepo, targetGitRepo
	}
	sourceRepo, sourceGitRepo, err := ownerTemplateRepo(ctx, doer, targetRepo, templatePath, paths)
	if err != nil {
		log.Error("ownerTemplateRepo: %v", err)
	}
	if sourceGitRepo == nil {
		return targetRepo, targetGitRepo
	}
	return sourceRepo, sourceGitRepo
}

func ownerTemplateRepo(ctx reqctx.RequestContext, doer *user_model.User, targetRepo *repo.Repository, templatePath string, paths []string) (*repo.Repository, *git.Repository, error) {
	for _, name := range []string{".gitea", ".github"} {
		sourceRepo, err := repo.GetRepositoryByName(ctx, targetRepo.OwnerID, name)
		if repo.IsErrRepoNotExist(err) {
			continue
		} else if err != nil {
			return nil, nil, err
		}
		if sourceRepo.IsEmpty {
			continue
		}
		sourceRepo.Owner = targetRepo.Owner
		perm, err := access_model.GetDoerRepoPermission(ctx, sourceRepo, doer)
		if err != nil {
			return nil, nil, err
		}
		if !perm.CanRead(unit.TypeCode) {
			continue
		}
		sourceGitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, sourceRepo)
		if err != nil {
			return nil, nil, err
		}
		if hasAnyPath(ctx, sourceRepo, sourceGitRepo, templatePath, paths) {
			return sourceRepo, sourceGitRepo, nil
		}
	}
	return nil, nil, nil
}

func hasAnyPath(ctx context.Context, repo *repo.Repository, gitRepo *git.Repository, templatePath string, paths []string) bool {
	if repo.IsEmpty {
		return false
	}
	commit, err := gitRepo.GetBranchCommit(ctx, repo.DefaultBranch)
	if err != nil {
		return false
	}
	exists := func(treePath string) bool {
		_, err := commit.GetTreeEntryByPath(ctx, gitRepo, treePath)
		return err == nil
	}
	return templatePath != "" && exists(templatePath) || slices.ContainsFunc(paths, exists)
}

func HasTemplatesOrContactLinks(ctx context.Context, repo *repo.Repository, gitRepo *git.Repository) bool {
	ret := ParseTemplatesFromDefaultBranch(ctx, repo, gitRepo)
	if len(ret.IssueTemplates) > 0 {
		return true
	}

	issueConfig, _ := GetTemplateConfigFromDefaultBranch(ctx, repo, gitRepo)
	return len(issueConfig.ContactLinks) > 0
}
