// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
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
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/repo"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/services/repository"

	"github.com/gobwas/glob"
)

const (
	tplProtectedBranch templates.TplName = "repo/settings/protected_branch"
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

	repo.PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplBranches)
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
	c.Data["Title"] = c.Locale.TrString("repo.settings.protected_branch") + " - " + rule.RuleName

	users, err := access_model.GetRepoReaders(c, c.Repo.Repository)
	if err != nil {
		c.ServerError("Repo.Repository.GetReaders", err)
		return
	}
	c.Data["Users"] = users
	c.Data["whitelist_users"] = strings.Join(base.Int64sToStrings(rule.WhitelistUserIDs), ",")
	c.Data["force_push_allowlist_users"] = strings.Join(base.Int64sToStrings(rule.ForcePushAllowlistUserIDs), ",")
	c.Data["merge_whitelist_users"] = strings.Join(base.Int64sToStrings(rule.MergeWhitelistUserIDs), ",")
	c.Data["approvals_whitelist_users"] = strings.Join(base.Int64sToStrings(rule.ApprovalsWhitelistUserIDs), ",")
	c.Data["status_check_contexts"] = strings.Join(rule.StatusCheckContexts, "\n")
	contexts, _ := git_model.FindRepoRecentCommitStatusContexts(c, c.Repo.Repository.ID, 7*24*time.Hour) // Find last week status check contexts
	c.Data["recent_status_checks"] = contexts

	if c.Repo.Owner.IsOrganization() {
		teams, err := organization.OrgFromUser(c.Repo.Owner).TeamsWithAccessToRepo(c, c.Repo.Repository.ID, perm.AccessModeRead)
		if err != nil {
			c.ServerError("Repo.Owner.TeamsWithAccessToRepo", err)
			return
		}
		c.Data["Teams"] = teams
		c.Data["whitelist_teams"] = strings.Join(base.Int64sToStrings(rule.WhitelistTeamIDs), ",")
		c.Data["force_push_allowlist_teams"] = strings.Join(base.Int64sToStrings(rule.ForcePushAllowlistTeamIDs), ",")
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

	var whitelistUsers, whitelistTeams, forcePushAllowlistUsers, forcePushAllowlistTeams, mergeWhitelistUsers, mergeWhitelistTeams, approvalsWhitelistUsers, approvalsWhitelistTeams []int64
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

	switch f.EnableForcePush {
	case "all":
		protectBranch.CanForcePush = true
		protectBranch.EnableForcePushAllowlist = false
		protectBranch.ForcePushAllowlistDeployKeys = false
	case "whitelist":
		protectBranch.CanForcePush = true
		protectBranch.EnableForcePushAllowlist = true
		protectBranch.ForcePushAllowlistDeployKeys = f.ForcePushAllowlistDeployKeys
		if strings.TrimSpace(f.ForcePushAllowlistUsers) != "" {
			forcePushAllowlistUsers, _ = base.StringsToInt64s(strings.Split(f.ForcePushAllowlistUsers, ","))
		}
		if strings.TrimSpace(f.ForcePushAllowlistTeams) != "" {
			forcePushAllowlistTeams, _ = base.StringsToInt64s(strings.Split(f.ForcePushAllowlistTeams, ","))
		}
	default:
		protectBranch.CanForcePush = false
		protectBranch.EnableForcePushAllowlist = false
		protectBranch.ForcePushAllowlistDeployKeys = false
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
	protectBranch.IgnoreStaleApprovals = f.IgnoreStaleApprovals
	protectBranch.RequireSignedCommits = f.RequireSignedCommits
	protectBranch.ProtectedFilePatterns = f.ProtectedFilePatterns
	protectBranch.UnprotectedFilePatterns = f.UnprotectedFilePatterns
	protectBranch.BlockOnOutdatedBranch = f.BlockOnOutdatedBranch
	protectBranch.BlockAdminMergeOverride = f.BlockAdminMergeOverride

	if err = pull_service.CreateOrUpdateProtectedBranch(ctx, ctx.Repo.Repository, protectBranch, git_model.WhitelistOptions{
		UserIDs:          whitelistUsers,
		TeamIDs:          whitelistTeams,
		ForcePushUserIDs: forcePushAllowlistUsers,
		ForcePushTeamIDs: forcePushAllowlistTeams,
		MergeUserIDs:     mergeWhitelistUsers,
		MergeTeamIDs:     mergeWhitelistTeams,
		ApprovalsUserIDs: approvalsWhitelistUsers,
		ApprovalsTeamIDs: approvalsWhitelistTeams,
	}); err != nil {
		ctx.ServerError("CreateOrUpdateProtectedBranch", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_protect_branch_success", protectBranch.RuleName))
	ctx.Redirect(fmt.Sprintf("%s/settings/branches?rule_name=%s", ctx.Repo.RepoLink, protectBranch.RuleName))
}

// DeleteProtectedBranchRulePost delete protected branch rule by id
func DeleteProtectedBranchRulePost(ctx *context.Context) {
	ruleID := ctx.PathParamInt64("id")
	if ruleID <= 0 {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", fmt.Sprintf("%d", ruleID)))
		ctx.JSONRedirect(fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink))
		return
	}

	rule, err := git_model.GetProtectedBranchRuleByID(ctx, ctx.Repo.Repository.ID, ruleID)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", fmt.Sprintf("%d", ruleID)))
		ctx.JSONRedirect(fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink))
		return
	}

	if rule == nil {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", fmt.Sprintf("%d", ruleID)))
		ctx.JSONRedirect(fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink))
		return
	}

	if err := git_model.DeleteProtectedBranch(ctx, ctx.Repo.Repository, ruleID); err != nil {
		ctx.Flash.Error(ctx.Tr("repo.settings.remove_protected_branch_failed", rule.RuleName))
		ctx.JSONRedirect(fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink))
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.remove_protected_branch_success", rule.RuleName))
	ctx.JSONRedirect(fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink))
}

func UpdateBranchProtectionPriories(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ProtectBranchPriorityForm)
	repo := ctx.Repo.Repository

	if err := git_model.UpdateProtectBranchPriorities(ctx, repo, form.IDs); err != nil {
		ctx.ServerError("UpdateProtectBranchPriorities", err)
		return
	}
}

// RenameBranchPost responses for rename a branch
func RenameBranchPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RenameBranchForm)

	if !ctx.Repo.CanCreateBranch() {
		ctx.NotFound(nil)
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
		return
	}

	msg, err := repository.RenameBranch(ctx, ctx.Repo.Repository, ctx.Doer, ctx.Repo.GitRepo, form.From, form.To)
	if err != nil {
		switch {
		case repo_model.IsErrUserDoesNotHaveAccessToRepo(err):
			ctx.Flash.Error(ctx.Tr("repo.branch.rename_default_or_protected_branch_error"))
			ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
		case git_model.IsErrBranchAlreadyExists(err):
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_already_exists", form.To))
			ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
		case errors.Is(err, git_model.ErrBranchIsProtected):
			ctx.Flash.Error(ctx.Tr("repo.branch.rename_protected_branch_failed"))
			ctx.Redirect(fmt.Sprintf("%s/branches", ctx.Repo.RepoLink))
		default:
			ctx.ServerError("RenameBranch", err)
		}
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
