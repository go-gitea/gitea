// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
	"github.com/gobwas/glob/syntax"
	"xorm.io/builder"
)

var ErrBranchIsProtected = errors.New("branch is protected")

// ProtectedBranch struct
type ProtectedBranch struct {
	ID                            int64                  `xorm:"pk autoincr"`
	RepoID                        int64                  `xorm:"UNIQUE(s)"`
	Repo                          *repo_model.Repository `xorm:"-"`
	RuleName                      string                 `xorm:"'branch_name' UNIQUE(s)"` // a branch name or a glob match to branch name
	Priority                      int64                  `xorm:"NOT NULL DEFAULT 0"`
	globRule                      glob.Glob              `xorm:"-"`
	isPlainName                   bool                   `xorm:"-"`
	CanPush                       bool                   `xorm:"NOT NULL DEFAULT false"`
	EnableWhitelist               bool
	WhitelistUserIDs              []int64  `xorm:"JSON TEXT"`
	WhitelistTeamIDs              []int64  `xorm:"JSON TEXT"`
	EnableMergeWhitelist          bool     `xorm:"NOT NULL DEFAULT false"`
	WhitelistDeployKeys           bool     `xorm:"NOT NULL DEFAULT false"`
	MergeWhitelistUserIDs         []int64  `xorm:"JSON TEXT"`
	MergeWhitelistTeamIDs         []int64  `xorm:"JSON TEXT"`
	CanForcePush                  bool     `xorm:"NOT NULL DEFAULT false"`
	EnableForcePushAllowlist      bool     `xorm:"NOT NULL DEFAULT false"`
	ForcePushAllowlistUserIDs     []int64  `xorm:"JSON TEXT"`
	ForcePushAllowlistTeamIDs     []int64  `xorm:"JSON TEXT"`
	ForcePushAllowlistDeployKeys  bool     `xorm:"NOT NULL DEFAULT false"`
	EnableStatusCheck             bool     `xorm:"NOT NULL DEFAULT false"`
	StatusCheckContexts           []string `xorm:"JSON TEXT"`
	EnableApprovalsWhitelist      bool     `xorm:"NOT NULL DEFAULT false"`
	ApprovalsWhitelistUserIDs     []int64  `xorm:"JSON TEXT"`
	ApprovalsWhitelistTeamIDs     []int64  `xorm:"JSON TEXT"`
	RequiredApprovals             int64    `xorm:"NOT NULL DEFAULT 0"`
	BlockOnRejectedReviews        bool     `xorm:"NOT NULL DEFAULT false"`
	BlockOnOfficialReviewRequests bool     `xorm:"NOT NULL DEFAULT false"`
	BlockOnOutdatedBranch         bool     `xorm:"NOT NULL DEFAULT false"`
	DismissStaleApprovals         bool     `xorm:"NOT NULL DEFAULT false"`
	IgnoreStaleApprovals          bool     `xorm:"NOT NULL DEFAULT false"`
	RequireSignedCommits          bool     `xorm:"NOT NULL DEFAULT false"`
	ProtectedFilePatterns         string   `xorm:"TEXT"`
	UnprotectedFilePatterns       string   `xorm:"TEXT"`
	BlockAdminMergeOverride       bool     `xorm:"NOT NULL DEFAULT false"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ProtectedBranch))
}

// IsRuleNameSpecial return true if it contains special character
func IsRuleNameSpecial(ruleName string) bool {
	for i := 0; i < len(ruleName); i++ {
		if syntax.Special(ruleName[i]) {
			return true
		}
	}
	return false
}

func (protectBranch *ProtectedBranch) loadGlob() {
	if protectBranch.isPlainName || protectBranch.globRule != nil {
		return
	}
	// detect if it is not glob
	if !IsRuleNameSpecial(protectBranch.RuleName) {
		protectBranch.isPlainName = true
		return
	}
	// now we load the glob
	var err error
	protectBranch.globRule, err = glob.Compile(protectBranch.RuleName, '/')
	if err != nil {
		log.Warn("Invalid glob rule for ProtectedBranch[%d]: %s %v", protectBranch.ID, protectBranch.RuleName, err)
		protectBranch.globRule = glob.MustCompile(glob.QuoteMeta(protectBranch.RuleName), '/')
	}
}

// Match tests if branchName matches the rule
func (protectBranch *ProtectedBranch) Match(branchName string) bool {
	protectBranch.loadGlob()
	if protectBranch.isPlainName {
		return strings.EqualFold(protectBranch.RuleName, branchName)
	}

	return protectBranch.globRule.Match(branchName)
}

func (protectBranch *ProtectedBranch) LoadRepo(ctx context.Context) (err error) {
	if protectBranch.Repo != nil {
		return nil
	}
	protectBranch.Repo, err = repo_model.GetRepositoryByID(ctx, protectBranch.RepoID)
	return err
}

// CanUserPush returns if some user could push to this protected branch
func (protectBranch *ProtectedBranch) CanUserPush(ctx context.Context, user *user_model.User) bool {
	if !protectBranch.CanPush {
		return false
	}

	if !protectBranch.EnableWhitelist {
		if err := protectBranch.LoadRepo(ctx); err != nil {
			log.Error("LoadRepo: %v", err)
			return false
		}

		writeAccess, err := access_model.HasAccessUnit(ctx, user, protectBranch.Repo, unit.TypeCode, perm.AccessModeWrite)
		if err != nil {
			log.Error("HasAccessUnit: %v", err)
			return false
		}
		return writeAccess
	}

	if slices.Contains(protectBranch.WhitelistUserIDs, user.ID) {
		return true
	}

	if len(protectBranch.WhitelistTeamIDs) == 0 {
		return false
	}

	in, err := organization.IsUserInTeams(ctx, user.ID, protectBranch.WhitelistTeamIDs)
	if err != nil {
		log.Error("IsUserInTeams: %v", err)
		return false
	}
	return in
}

// CanUserForcePush returns if some user could force push to this protected branch
// Since force-push extends normal push, we also check if user has regular push access
func (protectBranch *ProtectedBranch) CanUserForcePush(ctx context.Context, user *user_model.User) bool {
	if !protectBranch.CanForcePush {
		return false
	}

	if !protectBranch.EnableForcePushAllowlist {
		return protectBranch.CanUserPush(ctx, user)
	}

	if slices.Contains(protectBranch.ForcePushAllowlistUserIDs, user.ID) {
		return protectBranch.CanUserPush(ctx, user)
	}

	if len(protectBranch.ForcePushAllowlistTeamIDs) == 0 {
		return false
	}

	in, err := organization.IsUserInTeams(ctx, user.ID, protectBranch.ForcePushAllowlistTeamIDs)
	if err != nil {
		log.Error("IsUserInTeams: %v", err)
		return false
	}
	return in && protectBranch.CanUserPush(ctx, user)
}

// IsUserMergeWhitelisted checks if some user is whitelisted to merge to this branch
func IsUserMergeWhitelisted(ctx context.Context, protectBranch *ProtectedBranch, userID int64, permissionInRepo access_model.Permission) bool {
	if !protectBranch.EnableMergeWhitelist {
		// Then we need to fall back on whether the user has write permission
		return permissionInRepo.CanWrite(unit.TypeCode)
	}

	if slices.Contains(protectBranch.MergeWhitelistUserIDs, userID) {
		return true
	}

	if len(protectBranch.MergeWhitelistTeamIDs) == 0 {
		return false
	}

	in, err := organization.IsUserInTeams(ctx, userID, protectBranch.MergeWhitelistTeamIDs)
	if err != nil {
		log.Error("IsUserInTeams: %v", err)
		return false
	}
	return in
}

// IsUserOfficialReviewer check if user is official reviewer for the branch (counts towards required approvals)
func IsUserOfficialReviewer(ctx context.Context, protectBranch *ProtectedBranch, user *user_model.User) (bool, error) {
	repo, err := repo_model.GetRepositoryByID(ctx, protectBranch.RepoID)
	if err != nil {
		return false, err
	}

	if !protectBranch.EnableApprovalsWhitelist {
		// Anyone with write access is considered official reviewer
		writeAccess, err := access_model.HasAccessUnit(ctx, user, repo, unit.TypeCode, perm.AccessModeWrite)
		if err != nil {
			return false, err
		}
		return writeAccess, nil
	}

	if slices.Contains(protectBranch.ApprovalsWhitelistUserIDs, user.ID) {
		return true, nil
	}

	inTeam, err := organization.IsUserInTeams(ctx, user.ID, protectBranch.ApprovalsWhitelistTeamIDs)
	if err != nil {
		return false, err
	}

	return inTeam, nil
}

// GetProtectedFilePatterns parses a semicolon separated list of protected file patterns and returns a glob.Glob slice
func (protectBranch *ProtectedBranch) GetProtectedFilePatterns() []glob.Glob {
	return getFilePatterns(protectBranch.ProtectedFilePatterns)
}

// GetUnprotectedFilePatterns parses a semicolon separated list of unprotected file patterns and returns a glob.Glob slice
func (protectBranch *ProtectedBranch) GetUnprotectedFilePatterns() []glob.Glob {
	return getFilePatterns(protectBranch.UnprotectedFilePatterns)
}

func getFilePatterns(filePatterns string) []glob.Glob {
	extarr := make([]glob.Glob, 0, 10)
	for _, expr := range strings.Split(strings.ToLower(filePatterns), ";") {
		expr = strings.TrimSpace(expr)
		if expr != "" {
			if g, err := glob.Compile(expr, '.', '/'); err != nil {
				log.Info("Invalid glob expression '%s' (skipped): %v", expr, err)
			} else {
				extarr = append(extarr, g)
			}
		}
	}
	return extarr
}

// MergeBlockedByProtectedFiles returns true if merge is blocked by protected files change
func (protectBranch *ProtectedBranch) MergeBlockedByProtectedFiles(changedProtectedFiles []string) bool {
	glob := protectBranch.GetProtectedFilePatterns()
	if len(glob) == 0 {
		return false
	}

	return len(changedProtectedFiles) > 0
}

// IsProtectedFile return if path is protected
func (protectBranch *ProtectedBranch) IsProtectedFile(patterns []glob.Glob, path string) bool {
	if len(patterns) == 0 {
		patterns = protectBranch.GetProtectedFilePatterns()
		if len(patterns) == 0 {
			return false
		}
	}

	lpath := strings.ToLower(strings.TrimSpace(path))

	r := false
	for _, pat := range patterns {
		if pat.Match(lpath) {
			r = true
			break
		}
	}

	return r
}

// IsUnprotectedFile return if path is unprotected
func (protectBranch *ProtectedBranch) IsUnprotectedFile(patterns []glob.Glob, path string) bool {
	if len(patterns) == 0 {
		patterns = protectBranch.GetUnprotectedFilePatterns()
		if len(patterns) == 0 {
			return false
		}
	}

	lpath := strings.ToLower(strings.TrimSpace(path))

	r := false
	for _, pat := range patterns {
		if pat.Match(lpath) {
			r = true
			break
		}
	}

	return r
}

// GetProtectedBranchRuleByName getting protected branch rule by name
func GetProtectedBranchRuleByName(ctx context.Context, repoID int64, ruleName string) (*ProtectedBranch, error) {
	// branch_name is legacy name, it actually is rule name
	rel, exist, err := db.Get[ProtectedBranch](ctx, builder.Eq{"repo_id": repoID, "branch_name": ruleName})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, nil
	}
	return rel, nil
}

// GetProtectedBranchRuleByID getting protected branch rule by rule ID
func GetProtectedBranchRuleByID(ctx context.Context, repoID, ruleID int64) (*ProtectedBranch, error) {
	rel, exist, err := db.Get[ProtectedBranch](ctx, builder.Eq{"repo_id": repoID, "id": ruleID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, nil
	}
	return rel, nil
}

// WhitelistOptions represent all sorts of whitelists used for protected branches
type WhitelistOptions struct {
	UserIDs []int64
	TeamIDs []int64

	ForcePushUserIDs []int64
	ForcePushTeamIDs []int64

	MergeUserIDs []int64
	MergeTeamIDs []int64

	ApprovalsUserIDs []int64
	ApprovalsTeamIDs []int64
}

// UpdateProtectBranch saves branch protection options of repository.
// If ID is 0, it creates a new record. Otherwise, updates existing record.
// This function also performs check if whitelist user and team's IDs have been changed
// to avoid unnecessary whitelist delete and regenerate.
func UpdateProtectBranch(ctx context.Context, repo *repo_model.Repository, protectBranch *ProtectedBranch, opts WhitelistOptions) (err error) {
	err = repo.MustNotBeArchived()
	if err != nil {
		return err
	}

	if err = repo.LoadOwner(ctx); err != nil {
		return fmt.Errorf("LoadOwner: %v", err)
	}

	whitelist, err := updateUserWhitelist(ctx, repo, protectBranch.WhitelistUserIDs, opts.UserIDs)
	if err != nil {
		return err
	}
	protectBranch.WhitelistUserIDs = whitelist

	whitelist, err = updateUserWhitelist(ctx, repo, protectBranch.ForcePushAllowlistUserIDs, opts.ForcePushUserIDs)
	if err != nil {
		return err
	}
	protectBranch.ForcePushAllowlistUserIDs = whitelist

	whitelist, err = updateUserWhitelist(ctx, repo, protectBranch.MergeWhitelistUserIDs, opts.MergeUserIDs)
	if err != nil {
		return err
	}
	protectBranch.MergeWhitelistUserIDs = whitelist

	whitelist, err = updateApprovalWhitelist(ctx, repo, protectBranch.ApprovalsWhitelistUserIDs, opts.ApprovalsUserIDs)
	if err != nil {
		return err
	}
	protectBranch.ApprovalsWhitelistUserIDs = whitelist

	// if the repo is in an organization
	whitelist, err = updateTeamWhitelist(ctx, repo, protectBranch.WhitelistTeamIDs, opts.TeamIDs)
	if err != nil {
		return err
	}
	protectBranch.WhitelistTeamIDs = whitelist

	whitelist, err = updateTeamWhitelist(ctx, repo, protectBranch.ForcePushAllowlistTeamIDs, opts.ForcePushTeamIDs)
	if err != nil {
		return err
	}
	protectBranch.ForcePushAllowlistTeamIDs = whitelist

	whitelist, err = updateTeamWhitelist(ctx, repo, protectBranch.MergeWhitelistTeamIDs, opts.MergeTeamIDs)
	if err != nil {
		return err
	}
	protectBranch.MergeWhitelistTeamIDs = whitelist

	whitelist, err = updateTeamWhitelist(ctx, repo, protectBranch.ApprovalsWhitelistTeamIDs, opts.ApprovalsTeamIDs)
	if err != nil {
		return err
	}
	protectBranch.ApprovalsWhitelistTeamIDs = whitelist

	// Looks like it's a new rule
	if protectBranch.ID == 0 {
		// as it's a new rule and if priority was not set, we need to calc it.
		if protectBranch.Priority == 0 {
			var lowestPrio int64
			// because of mssql we can not use builder or save xorm syntax, so raw sql it is
			if _, err := db.GetEngine(ctx).SQL(`SELECT MAX(priority) FROM protected_branch WHERE repo_id = ?`, protectBranch.RepoID).
				Get(&lowestPrio); err != nil {
				return err
			}
			log.Trace("Create new ProtectedBranch at repo[%d] and detect current lowest priority '%d'", protectBranch.RepoID, lowestPrio)
			protectBranch.Priority = lowestPrio + 1
		}

		if _, err = db.GetEngine(ctx).Insert(protectBranch); err != nil {
			return fmt.Errorf("Insert: %v", err)
		}
		return nil
	}

	// update the rule
	if _, err = db.GetEngine(ctx).ID(protectBranch.ID).AllCols().Update(protectBranch); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	return nil
}

func UpdateProtectBranchPriorities(ctx context.Context, repo *repo_model.Repository, ids []int64) error {
	prio := int64(1)
	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, id := range ids {
			if _, err := db.GetEngine(ctx).
				ID(id).Where("repo_id = ?", repo.ID).
				Cols("priority").
				Update(&ProtectedBranch{
					Priority: prio,
				}); err != nil {
				return err
			}
			prio++
		}
		return nil
	})
}

// updateApprovalWhitelist checks whether the user whitelist changed and returns a whitelist with
// the users from newWhitelist which have explicit read or write access to the repo.
func updateApprovalWhitelist(ctx context.Context, repo *repo_model.Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasUsersChanged := !util.SliceSortedEqual(currentWhitelist, newWhitelist)
	if !hasUsersChanged {
		return currentWhitelist, nil
	}

	whitelist = make([]int64, 0, len(newWhitelist))
	for _, userID := range newWhitelist {
		if reader, err := access_model.IsRepoReader(ctx, repo, userID); err != nil {
			return nil, err
		} else if !reader {
			continue
		}
		whitelist = append(whitelist, userID)
	}

	return whitelist, err
}

// updateUserWhitelist checks whether the user whitelist changed and returns a whitelist with
// the users from newWhitelist which have write access to the repo.
func updateUserWhitelist(ctx context.Context, repo *repo_model.Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasUsersChanged := !util.SliceSortedEqual(currentWhitelist, newWhitelist)
	if !hasUsersChanged {
		return currentWhitelist, nil
	}

	whitelist = make([]int64, 0, len(newWhitelist))
	for _, userID := range newWhitelist {
		user, err := user_model.GetUserByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("GetUserByID [user_id: %d, repo_id: %d]: %v", userID, repo.ID, err)
		}
		perm, err := access_model.GetUserRepoPermission(ctx, repo, user)
		if err != nil {
			return nil, fmt.Errorf("GetUserRepoPermission [user_id: %d, repo_id: %d]: %v", userID, repo.ID, err)
		}

		if !perm.CanWrite(unit.TypeCode) {
			continue // Drop invalid user ID
		}

		whitelist = append(whitelist, userID)
	}

	return whitelist, err
}

// updateTeamWhitelist checks whether the team whitelist changed and returns a whitelist with
// the teams from newWhitelist which have write access to the repo.
func updateTeamWhitelist(ctx context.Context, repo *repo_model.Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasTeamsChanged := !util.SliceSortedEqual(currentWhitelist, newWhitelist)
	if !hasTeamsChanged {
		return currentWhitelist, nil
	}

	teams, err := organization.GetTeamsWithAccessToRepo(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead)
	if err != nil {
		return nil, fmt.Errorf("GetTeamsWithAccessToRepo [org_id: %d, repo_id: %d]: %v", repo.OwnerID, repo.ID, err)
	}

	whitelist = make([]int64, 0, len(teams))
	for i := range teams {
		if slices.Contains(newWhitelist, teams[i].ID) {
			whitelist = append(whitelist, teams[i].ID)
		}
	}

	return whitelist, err
}

// DeleteProtectedBranch removes ProtectedBranch relation between the user and repository.
func DeleteProtectedBranch(ctx context.Context, repo *repo_model.Repository, id int64) (err error) {
	err = repo.MustNotBeArchived()
	if err != nil {
		return err
	}

	protectedBranch := &ProtectedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	if affected, err := db.GetEngine(ctx).Delete(protectedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("delete protected branch ID(%v) failed", id)
	}

	return nil
}

// removeIDsFromProtectedBranch is a helper function to remove IDs from protected branch options
func removeIDsFromProtectedBranch(ctx context.Context, p *ProtectedBranch, userID, teamID int64, columnNames []string) error {
	lenUserIDs, lenForcePushIDs, lenApprovalIDs, lenMergeIDs := len(p.WhitelistUserIDs), len(p.ForcePushAllowlistUserIDs), len(p.ApprovalsWhitelistUserIDs), len(p.MergeWhitelistUserIDs)
	lenTeamIDs, lenForcePushTeamIDs, lenApprovalTeamIDs, lenMergeTeamIDs := len(p.WhitelistTeamIDs), len(p.ForcePushAllowlistTeamIDs), len(p.ApprovalsWhitelistTeamIDs), len(p.MergeWhitelistTeamIDs)

	if userID > 0 {
		p.WhitelistUserIDs = util.SliceRemoveAll(p.WhitelistUserIDs, userID)
		p.ForcePushAllowlistUserIDs = util.SliceRemoveAll(p.ForcePushAllowlistUserIDs, userID)
		p.ApprovalsWhitelistUserIDs = util.SliceRemoveAll(p.ApprovalsWhitelistUserIDs, userID)
		p.MergeWhitelistUserIDs = util.SliceRemoveAll(p.MergeWhitelistUserIDs, userID)
	}

	if teamID > 0 {
		p.WhitelistTeamIDs = util.SliceRemoveAll(p.WhitelistTeamIDs, teamID)
		p.ForcePushAllowlistTeamIDs = util.SliceRemoveAll(p.ForcePushAllowlistTeamIDs, teamID)
		p.ApprovalsWhitelistTeamIDs = util.SliceRemoveAll(p.ApprovalsWhitelistTeamIDs, teamID)
		p.MergeWhitelistTeamIDs = util.SliceRemoveAll(p.MergeWhitelistTeamIDs, teamID)
	}

	if (lenUserIDs != len(p.WhitelistUserIDs) ||
		lenForcePushIDs != len(p.ForcePushAllowlistUserIDs) ||
		lenApprovalIDs != len(p.ApprovalsWhitelistUserIDs) ||
		lenMergeIDs != len(p.MergeWhitelistUserIDs)) ||
		(lenTeamIDs != len(p.WhitelistTeamIDs) ||
			lenForcePushTeamIDs != len(p.ForcePushAllowlistTeamIDs) ||
			lenApprovalTeamIDs != len(p.ApprovalsWhitelistTeamIDs) ||
			lenMergeTeamIDs != len(p.MergeWhitelistTeamIDs)) {
		if _, err := db.GetEngine(ctx).ID(p.ID).Cols(columnNames...).Update(p); err != nil {
			return fmt.Errorf("updateProtectedBranches: %v", err)
		}
	}
	return nil
}

// RemoveUserIDFromProtectedBranch removes all user ids from protected branch options
func RemoveUserIDFromProtectedBranch(ctx context.Context, p *ProtectedBranch, userID int64) error {
	columnNames := []string{
		"whitelist_user_i_ds",
		"force_push_allowlist_user_i_ds",
		"merge_whitelist_user_i_ds",
		"approvals_whitelist_user_i_ds",
	}
	return removeIDsFromProtectedBranch(ctx, p, userID, 0, columnNames)
}

// RemoveTeamIDFromProtectedBranch removes all team ids from protected branch options
func RemoveTeamIDFromProtectedBranch(ctx context.Context, p *ProtectedBranch, teamID int64) error {
	columnNames := []string{
		"whitelist_team_i_ds",
		"force_push_allowlist_team_i_ds",
		"merge_whitelist_team_i_ds",
		"approvals_whitelist_team_i_ds",
	}
	return removeIDsFromProtectedBranch(ctx, p, 0, teamID, columnNames)
}
