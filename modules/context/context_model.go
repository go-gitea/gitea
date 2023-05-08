// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"path"
	"strings"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/issue/template"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// IsUserSiteAdmin returns true if current user is a site admin
func (ctx *Context) IsUserSiteAdmin() bool {
	return ctx.IsSigned && ctx.Doer.IsAdmin
}

// IsUserRepoOwner returns true if current user owns current repo
func (ctx *Context) IsUserRepoOwner() bool {
	return ctx.Repo.IsOwner()
}

// IsUserRepoAdmin returns true if current user is admin in current repo
func (ctx *Context) IsUserRepoAdmin() bool {
	return ctx.Repo.IsAdmin()
}

// IsUserRepoWriter returns true if current user has write privilege in current repo
func (ctx *Context) IsUserRepoWriter(unitTypes []unit.Type) bool {
	for _, unitType := range unitTypes {
		if ctx.Repo.CanWrite(unitType) {
			return true
		}
	}

	return false
}

// IsUserRepoReaderSpecific returns true if current user can read current repo's specific part
func (ctx *Context) IsUserRepoReaderSpecific(unitType unit.Type) bool {
	return ctx.Repo.CanRead(unitType)
}

// IsUserRepoReaderAny returns true if current user can read any part of current repo
func (ctx *Context) IsUserRepoReaderAny() bool {
	return ctx.Repo.HasAccess()
}

// IssueTemplatesFromDefaultBranch checks for valid issue templates in the repo's default branch,
func (ctx *Context) IssueTemplatesFromDefaultBranch() []*api.IssueTemplate {
	ret, _ := ctx.IssueTemplatesErrorsFromDefaultBranch()
	return ret
}

// IssueTemplatesErrorsFromDefaultBranch checks for issue templates in the repo's default branch,
// returns valid templates and the errors of invalid template files.
func (ctx *Context) IssueTemplatesErrorsFromDefaultBranch() ([]*api.IssueTemplate, map[string]error) {
	var issueTemplates []*api.IssueTemplate

	if ctx.Repo.Repository.IsEmpty {
		return issueTemplates, nil
	}

	if ctx.Repo.Commit == nil {
		var err error
		ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			return issueTemplates, nil
		}
	}

	invalidFiles := map[string]error{}
	for _, dirName := range IssueTemplateDirCandidates {
		tree, err := ctx.Repo.Commit.SubTree(dirName)
		if err != nil {
			log.Debug("get sub tree of %s: %v", dirName, err)
			continue
		}
		entries, err := tree.ListEntries()
		if err != nil {
			log.Debug("list entries in %s: %v", dirName, err)
			return issueTemplates, nil
		}
		for _, entry := range entries {
			if !template.CouldBe(entry.Name()) {
				continue
			}
			fullName := path.Join(dirName, entry.Name())
			if it, err := template.UnmarshalFromEntry(entry, dirName); err != nil {
				invalidFiles[fullName] = err
			} else {
				if !strings.HasPrefix(it.Ref, "refs/") { // Assume that the ref intended is always a branch - for tags users should use refs/tags/<ref>
					it.Ref = git.BranchPrefix + it.Ref
				}
				issueTemplates = append(issueTemplates, it)
			}
		}
	}
	return issueTemplates, invalidFiles
}

// IssueConfigFromDefaultBranch returns the issue config for this repo.
// It never returns a nil config.
func (ctx *Context) IssueConfigFromDefaultBranch() (api.IssueConfig, error) {
	if ctx.Repo.Repository.IsEmpty {
		return GetDefaultIssueConfig(), nil
	}

	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		return GetDefaultIssueConfig(), err
	}

	for _, configName := range IssueConfigCandidates {
		if _, err := commit.GetTreeEntryByPath(configName + ".yaml"); err == nil {
			return ctx.Repo.GetIssueConfig(configName+".yaml", commit)
		}

		if _, err := commit.GetTreeEntryByPath(configName + ".yml"); err == nil {
			return ctx.Repo.GetIssueConfig(configName+".yml", commit)
		}
	}

	return GetDefaultIssueConfig(), nil
}

func (ctx *Context) HasIssueTemplatesOrContactLinks() bool {
	if len(ctx.IssueTemplatesFromDefaultBranch()) > 0 {
		return true
	}

	issueConfig, _ := ctx.IssueConfigFromDefaultBranch()
	return len(issueConfig.ContactLinks) > 0
}
