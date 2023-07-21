// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/services/repository"

	"github.com/gobwas/glob"
)

const (
	tplProtectedBranch base.TplName = "repo/settings/protected_branch"
)

// ProtectedBranchRules render the page to protect the repository
func ProtectedBranchRules(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.branches")
	ctx.Data["PageIsSettingsBranches"] = true

	rules, err := git_model.FindRepoProtectedBranchRules(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetProtectedBranches", err)
		return
	}
	ctx.Data["ProtectedBranches"] = rules

	ctx.HTML(http.StatusOK, tplBranches)
}

// SetDefaultBranchPost set default branch
func SetDefaultBranchPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.branches.update_default_branch")
	ctx.Data["PageIsSettingsBranches"] = true

	repo := ctx.Repo.Repository

	switch ctx.FormString("action") {
	case "default_branch":
		if ctx.HasError() {
			ctx.HTML(http.StatusOK, tplBranches)
			return
		}

		branch := ctx.FormString("branch")
		if !ctx.Repo.GitRepo.IsBranchExist(branch) {
			ctx.Status(http.StatusNotFound)
			return
		} else if repo.DefaultBranch != branch {
			repo.DefaultBranch = branch
			if err := ctx.Repo.GitRepo.SetDefaultBranch(branch); err != nil {
				if !git.IsErrUnsupportedVersion(err) {
					ctx.ServerError("SetDefaultBranch", err)
					return
				}
			}
			if err := repo_model.UpdateDefaultBranch(repo); err != nil {
				ctx.ServerError("SetDefaultBranch", err)
				return
			}
		}

		log.Trace("Repository basic settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
	default:
		ctx.NotFound("", nil)
	}
}

// SettingsProtectedBranch renders the protected branch setting page
func SettingsProtectedBranch(c *context.Context) {
	ruleName := c.FormString("rule_name")
	var rule *git_model.ProtectedBranch
	if ruleName != "" {
		var err error
		rule, err = git_model.GetProtectedBranchRuleByName(c, c.Repo.Repository.ID, ruleName)
		if err != nil {
			c.ServerError("GetProtectBranchOfRepoByName", err)
			return
		}
	}

	if rule == nil {
		// No options found, create defaults.
		rule = &git_model.ProtectedBranch{}
	}

	c.Data["PageIsSettingsBranches"] = true
	c.Data["Title"] = c.Tr("repo.settings.protected_branch") + " - " + rule.RuleName

	users, err := access_model.GetRepoReaders(c.Repo.Repository)
	if err != nil {
		c.ServerError("Repo.Repository.GetReaders", err)
		return
	}
	c.Data["Users"] = users
	c.Data["whitelist_users"] = strings.Join(base.Int64sToStrings(rule.WhitelistUserIDs), ",")
	c.Data["merge_whitelist_users"] = strings.Join(base.Int64sToStrings(rule.MergeWhitelistUserIDs), ",")
	c.Data["approvals_whitelist_users"] = strings.Join(base.Int64sToStrings(rule.ApprovalsWhitelistUserIDs), ",")
	c.Data["status_check_contexts"] = strings.Join(rule.StatusCheckContexts, "\n")
	contexts, _ := git_model.FindRepoRecentCommitStatusContexts(c, c.Repo.Repository.ID, 7*24*time.Hour) // Find last week status check contexts
	c.Data["recent_status_checks"] = contexts

	if c.Repo.Owner.IsOrganization() {
		teams, err := organization.OrgFromUser(c.Repo.Owner).TeamsWithAccessToRepo(c.Repo.Repository.ID, perm.AccessModeRead)
		if err != nil {
			c.ServerError("Repo.Owner.TeamsWithAccessToRepo", err)
			return
		}
		c.Data["Teams"] = teams
		c.Data["whitelist_teams"] = strings.Join(base.Int64sToStrings(rule.WhitelistTeamIDs), ",")
		c.Data["merge_whitelist_teams"] = strings.Join(base.Int64sToStrings(rule.MergeWhitelistTeamIDs), ",")
		c.Data["approvals_whitelist_teams"] = strings.Join(base.Int64sToStrings(rule.ApprovalsWhitelistTeamIDs), ",")
	}

	c.Data["Rule"] = rule
	c.HTML(http.StatusOK, tplProtectedBranch)
}

// SettingsProtectedBranchPost updates the protected branch settings
func SettingsProtectedBranchPost(ctx *context.Context) {
	f := web.GetForm(ctx).(*forms.ProtectBranchForm)
	var protectBranch *git_model.ProtectedBranch
	if f.RuleName == "" {
		ctx.Flash.Error(ctx.Tr("repo.settings.protected_branch_required_rule_name"))
		ctx.Redirect(fmt.Sprintf("%s/settings/branches/edit", ctx.Repo.RepoLink))
		return
	}

	var err error
	if f.RuleID > 0 {
		// If the RuleID isn't 0, it must be an edit operation. So we get rule by id.
		protectBranch, err = git_model.GetProtectedBranchRuleByID(ctx, ctx.Repo.Repository.ID, f.RuleID)
		if err != nil {
			ctx.ServerError("GetProtectBranchOfRepoByID", err)
			return
		}
		if protectBranch != nil && protectBranch.RuleName != f.RuleName {
			// RuleName changed. We need to check if there is a rule with the same name.
			// If a rule with the same name exists, an error should be returned.
			sameNameProtectBranch, err := git_model.GetProtectedBranchRuleByName(ctx, ctx.Repo.Repository.ID, f.RuleName)
			if err != nil {
				ctx.ServerError("GetProtectBranchOfRepoByName", err)
				return
			}
			if sameNameProtectBranch != nil {
				ctx.Flash.Error(ctx.Tr("repo.settings.protected_branch_duplicate_rule_name"))
				ctx.Redirect(fmt.Sprintf("%s/settings/branches/edit?rule_name=%s", ctx.Repo.RepoLink, protectBranch.RuleName))
				return
			}
		}
	} else {
		// FIXME: If a new ProtectBranch has a duplicate RuleName, an error should be returned.
		// Currently, if a new ProtectBranch with a duplicate RuleName is created, the existing ProtectBranch will be updated.
		// But we cannot modify this logic now because many unit tests rely on it.
		protectBranch, err = git_model.GetProtectedBranchRuleByName(ctx, ctx.Repo.Repository.ID, f.RuleName)
		if err != nil {
			ctx.ServerError("GetProtectBranchOfRepoByName", err)
			return
		}
	}
	if protectBranch == nil {
		// No options found, create defaults.
		protectBranch = &git_model.ProtectedBranch{
			RepoID:   ctx.Repo.Repository.ID,
			RuleName: f.RuleName,
		}
	}

	var whitelistUsers, whitelistTeams, mergeWhitelistUsers, mergeWhitelistTeams, approvalsWhitelistUsers, approvalsWhitelistTeams []int64
	protectBranch.RuleName = f.RuleName
	if f.RequiredApprovals < 0 {
		ctx.Flash.Error(ctx.Tr("repo.settings.protected_branch_required_approvals_min"))
		ctx.Redirect(fmt.Sprintf("%s/settings/branches/edit?rule_name=%s", ctx.Repo.RepoLink, f.RuleName))
		return
	}

	switch f.EnablePush {
	case "all":
		protectBranch.CanPush = true
		protectBranch.EnableWhitelist = false
		protectBranch.WhitelistDeployKeys = false
	case "whitelist":
		protectBranch.CanPush = true
		protectBranch.EnableWhitelist = true
		protectBranch.WhitelistDeployKeys = f.WhitelistDeployKeys
		if strings.TrimSpace(f.WhitelistUsers) != "" {
			whitelistUsers, _ = base.StringsToInt64s(strings.Split(f.WhitelistUsers, ","))
		}
		if strings.TrimSpace(f.WhitelistTeams) != "" {
			whitelistTeams, _ = base.StringsToInt64s(strings.Split(f.WhitelistTeams, ","))
		}
	default:
		protectBranch.CanPush = false
		protectBranch.EnableWhitelist = false
		protectBranch.WhitelistDeployKeys = false
	}

	protectBranch.EnableMergeWhitelist = f.EnableMergeWhitelist
	if f.EnableMergeWhitelist {
		if strings.TrimSpace(f.MergeWhitelistUsers) != "" {
			mergeWhitelistUsers, _ = base.StringsToInt64s(strings.Split(f.MergeWhitelistUsers, ","))
		}
		if strings.TrimSpace(f.MergeWhitelistTeams) != "" {
			mergeWhitelistTeams, _ = base.StringsToInt64s(strings.Split(f.MergeWhitelistTeams, ","))
		}
	}

	protectBranch.EnableStatusCheck = f.EnableStatusCheck
	if f.EnableStatusCheck {
		patterns := strings.Split(strings.ReplaceAll(f.StatusCheckContexts, "\r", "\n"), "\n")
		validPatterns := make([]string, 0, len(patterns))
		for _, pattern := range patterns {
			trimmed := strings.TrimSpace(pattern)
			if trimmed == "" {
				continue
			}
			if _, err := glob.Compile(trimmed); err != nil {
				ctx.Flash.Error(ctx.Tr("repo.settings.protect_invalid_status_check_pattern", pattern))
				ctx.Redirect(fmt.Sprintf("%s/settings/branches/edit?rule_name=%s", ctx.Repo.RepoLink, url.QueryEscape(protectBranch.RuleName)))
				return
			}
			validPatterns = append(validPatterns, trimmed)
		}
		if len(validPatterns) == 0 {
			// if status check is enabled, patterns slice is not allowed to be empty
			ctx.Flash.Error(ctx.Tr("repo.settings.protect_no_valid_status_check_patterns"))
			ctx.Redirect(fmt.Sprintf("%s/settings/branches/edit?rule_name=%s", ctx.Repo.RepoLink, url.QueryEscape(protectBranch.RuleName)))
			return
		}
		protectBranch.StatusCheckContexts = validPatterns
	} else {
		protectBranch.StatusCheckContexts = nil
	}

	protectBranch.RequiredApprovals = f.RequiredApprovals
	protectBranch.EnableApprovalsWhitelist = f.EnableApprovalsWhitelist
	if f.EnableApprovalsWhitelist {
		if strings.TrimSpace(f.ApprovalsWhitelistUsers) != "" {
			approvalsWhitelistUsers, _ = base.StringsToInt64s(strings.Split(f.ApprovalsWhitelistUsers, ","))
		}
		if strings.TrimSpace(f.ApprovalsWhitelistTeams) != "" {
			approvalsWhitelistTeams, _ = base.StringsToInt64s(strings.Split(f.ApprovalsWhitelistTeams, ","))
		}
	}
	protectBranch.BlockOnRejectedReviews = f.BlockOnRejectedReviews
	protectBranch.BlockOnOfficialReviewRequests = f.BlockOnOfficialReviewRequests
	protectBranch.DismissStaleApprovals = f.DismissStaleApprovals
	protectBranch.RequireSignedCommits = f.RequireSignedCommits
	protectBranch.ProtectedFilePatterns = f.ProtectedFilePatterns
	protectBranch.UnprotectedFilePatterns = f.UnprotectedFilePatterns
	protectBranch.BlockOnOutdatedBranch = f.BlockOnOutdatedBranch

	err = git_model.UpdateProtectBranch(ctx, ctx.Repo.Repository, protectBranch, git_model.WhitelistOptions{
		UserIDs:          whitelistUsers,
		TeamIDs:          whitelistTeams,
		MergeUserIDs:     mergeWhitelistUsers,
		MergeTeamIDs:     mergeWhitelistTeams,
		ApprovalsUserIDs: approvalsWhitelistUsers,
		ApprovalsTeamIDs: approvalsWhitelistTeams,
	})
	if err != nil {
		ctx.ServerError("UpdateProtectBranch", err)
		return
	}

	// FIXME: since we only need to recheck files protected rules, we could improve this
	matchedBranches, err := git_model.FindAllMatchedBranches(ctx, ctx.Repo.Repository.ID, protectBranch.RuleName)
	if err != nil {
		ctx.ServerError("FindAllMatchedBranches", err)
		return
	}
	for _, branchName := range matchedBranches {
		if err = pull_service.CheckPRsForBaseBranch(ctx.Repo.Repository, branchName); err != nil {
			ctx.ServerError("CheckPRsForBaseBranch", err)
			return
		}
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_protect_branch_success", protectBranch.RuleName))
	ctx.Redirect(fmt.Sprintf("%s/settings/branches?rule_name=%s", ctx.Repo.RepoLink, protectBranch.RuleName))
}

// DeleteProtectedBranchRulePost delete protected branch rule by id
func DeleteProtectedBranchRulePost(ctx *context.Context) {
	ruleID := ctx.ParamsInt64("id")
	if ruleID <= 0 {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", fmt.Sprintf("%d", ruleID)))
		ctx.JSON(http.StatusOK, map[string]any{
			"redirect": fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink),
		})
		return
	}

	rule, err := git_model.GetProtectedBranchRuleByID(ctx, ctx.Repo.Repository.ID, ruleID)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", fmt.Sprintf("%d", ruleID)))
		ctx.JSON(http.StatusOK, map[string]any{
			"redirect": fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink),
		})
		return
	}

	if rule == nil {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", fmt.Sprintf("%d", ruleID)))
		ctx.JSON(http.StatusOK, map[string]any{
			"redirect": fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink),
		})
		return
	}

	if err := git_model.DeleteProtectedBranch(ctx, ctx.Repo.Repository.ID, ruleID); err != nil {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", rule.RuleName))
		ctx.JSON(http.StatusOK, map[string]any{
			"redirect": fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink),
		})
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.remove_protected_branch_success", rule.RuleName))
	ctx.JSON(http.StatusOK, map[string]any{
		"redirect": fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink),
	})
}

// RenameBranchPost responses for rename a branch
func RenameBranchPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RenameBranchForm)

	if !ctx.Repo.CanCreateBranch() {
		ctx.NotFound("RenameBranch", nil)
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
		return
	}

	msg, err := repository.RenameBranch(ctx, ctx.Repo.Repository, ctx.Doer, ctx.Repo.GitRepo, form.From, form.To)
	if err != nil {
		ctx.ServerError("RenameBranch", err)
		return
	}

	if msg == "target_exist" {
		ctx.Flash.Error(ctx.Tr("repo.settings.rename_branch_failed_exist", form.To))
		ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
		return
	}

	if msg == "from_not_exist" {
		ctx.Flash.Error(ctx.Tr("repo.settings.rename_branch_failed_not_exist", form.From))
		ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.rename_branch_success", form.From, form.To))
	ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
}
