// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
)

// ProtectedBranch struct
type ProtectedBranch struct {
	ID                            int64  `xorm:"pk autoincr"`
	RepoID                        int64  `xorm:"UNIQUE(s)"`
	BranchName                    string `xorm:"UNIQUE(s)"`
	CanPush                       bool   `xorm:"NOT NULL DEFAULT false"`
	EnableWhitelist               bool
	WhitelistUserIDs              []int64  `xorm:"JSON TEXT"`
	WhitelistTeamIDs              []int64  `xorm:"JSON TEXT"`
	EnableMergeWhitelist          bool     `xorm:"NOT NULL DEFAULT false"`
	WhitelistDeployKeys           bool     `xorm:"NOT NULL DEFAULT false"`
	MergeWhitelistUserIDs         []int64  `xorm:"JSON TEXT"`
	MergeWhitelistTeamIDs         []int64  `xorm:"JSON TEXT"`
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
	RequireSignedCommits          bool     `xorm:"NOT NULL DEFAULT false"`
	ProtectedFilePatterns         string   `xorm:"TEXT"`
	UnprotectedFilePatterns       string   `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ProtectedBranch))
	db.RegisterModel(new(DeletedBranch))
	db.RegisterModel(new(RenamedBranch))
}

// IsProtected returns if the branch is protected
func (protectBranch *ProtectedBranch) IsProtected() bool {
	return protectBranch.ID > 0
}

// CanUserPush returns if some user could push to this protected branch
func (protectBranch *ProtectedBranch) CanUserPush(userID int64) bool {
	if !protectBranch.CanPush {
		return false
	}

	if !protectBranch.EnableWhitelist {
		if user, err := user_model.GetUserByID(userID); err != nil {
			log.Error("GetUserByID: %v", err)
			return false
		} else if repo, err := repo_model.GetRepositoryByID(protectBranch.RepoID); err != nil {
			log.Error("repo_model.GetRepositoryByID: %v", err)
			return false
		} else if writeAccess, err := access_model.HasAccessUnit(db.DefaultContext, user, repo, unit.TypeCode, perm.AccessModeWrite); err != nil {
			log.Error("HasAccessUnit: %v", err)
			return false
		} else {
			return writeAccess
		}
	}

	if base.Int64sContains(protectBranch.WhitelistUserIDs, userID) {
		return true
	}

	if len(protectBranch.WhitelistTeamIDs) == 0 {
		return false
	}

	in, err := organization.IsUserInTeams(db.DefaultContext, userID, protectBranch.WhitelistTeamIDs)
	if err != nil {
		log.Error("IsUserInTeams: %v", err)
		return false
	}
	return in
}

// IsUserMergeWhitelisted checks if some user is whitelisted to merge to this branch
func IsUserMergeWhitelisted(ctx context.Context, protectBranch *ProtectedBranch, userID int64, permissionInRepo access_model.Permission) bool {
	if !protectBranch.EnableMergeWhitelist {
		// Then we need to fall back on whether the user has write permission
		return permissionInRepo.CanWrite(unit.TypeCode)
	}

	if base.Int64sContains(protectBranch.MergeWhitelistUserIDs, userID) {
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
func IsUserOfficialReviewer(protectBranch *ProtectedBranch, user *user_model.User) (bool, error) {
	return isUserOfficialReviewer(db.DefaultContext, protectBranch, user)
}

func isUserOfficialReviewer(ctx context.Context, protectBranch *ProtectedBranch, user *user_model.User) (bool, error) {
	repo, err := repo_model.GetRepositoryByIDCtx(ctx, protectBranch.RepoID)
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

	if base.Int64sContains(protectBranch.ApprovalsWhitelistUserIDs, user.ID) {
		return true, nil
	}

	inTeam, err := organization.IsUserInTeams(ctx, user.ID, protectBranch.ApprovalsWhitelistTeamIDs)
	if err != nil {
		return false, err
	}

	return inTeam, nil
}

// HasEnoughApprovals returns true if pr has enough granted approvals.
func (protectBranch *ProtectedBranch) HasEnoughApprovals(ctx context.Context, pr *PullRequest) bool {
	if protectBranch.RequiredApprovals == 0 {
		return true
	}
	return protectBranch.GetGrantedApprovalsCount(ctx, pr) >= protectBranch.RequiredApprovals
}

// GetGrantedApprovalsCount returns the number of granted approvals for pr. A granted approval must be authored by a user in an approval whitelist.
func (protectBranch *ProtectedBranch) GetGrantedApprovalsCount(ctx context.Context, pr *PullRequest) int64 {
	sess := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeApprove).
		And("official = ?", true).
		And("dismissed = ?", false)
	if protectBranch.DismissStaleApprovals {
		sess = sess.And("stale = ?", false)
	}
	approvals, err := sess.Count(new(Review))
	if err != nil {
		log.Error("GetGrantedApprovalsCount: %v", err)
		return 0
	}

	return approvals
}

// MergeBlockedByRejectedReview returns true if merge is blocked by rejected reviews
func (protectBranch *ProtectedBranch) MergeBlockedByRejectedReview(ctx context.Context, pr *PullRequest) bool {
	if !protectBranch.BlockOnRejectedReviews {
		return false
	}
	rejectExist, err := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeReject).
		And("official = ?", true).
		And("dismissed = ?", false).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByRejectedReview: %v", err)
		return true
	}

	return rejectExist
}

// MergeBlockedByOfficialReviewRequests block merge because of some review request to official reviewer
// of from official review
func (protectBranch *ProtectedBranch) MergeBlockedByOfficialReviewRequests(ctx context.Context, pr *PullRequest) bool {
	if !protectBranch.BlockOnOfficialReviewRequests {
		return false
	}
	has, err := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeRequest).
		And("official = ?", true).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByOfficialReviewRequests: %v", err)
		return true
	}

	return has
}

// MergeBlockedByOutdatedBranch returns true if merge is blocked by an outdated head branch
func (protectBranch *ProtectedBranch) MergeBlockedByOutdatedBranch(pr *PullRequest) bool {
	return protectBranch.BlockOnOutdatedBranch && pr.CommitsBehind > 0
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
func (protectBranch *ProtectedBranch) MergeBlockedByProtectedFiles(pr *PullRequest) bool {
	glob := protectBranch.GetProtectedFilePatterns()
	if len(glob) == 0 {
		return false
	}

	return len(pr.ChangedProtectedFiles) > 0
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

// GetProtectedBranchBy getting protected branch by ID/Name
func GetProtectedBranchBy(repoID int64, branchName string) (*ProtectedBranch, error) {
	return getProtectedBranchBy(db.GetEngine(db.DefaultContext), repoID, branchName)
}

func getProtectedBranchBy(e db.Engine, repoID int64, branchName string) (*ProtectedBranch, error) {
	rel := &ProtectedBranch{RepoID: repoID, BranchName: branchName}
	has, err := e.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rel, nil
}

// WhitelistOptions represent all sorts of whitelists used for protected branches
type WhitelistOptions struct {
	UserIDs []int64
	TeamIDs []int64

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
	if err = repo.GetOwner(ctx); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}

	whitelist, err := updateUserWhitelist(ctx, repo, protectBranch.WhitelistUserIDs, opts.UserIDs)
	if err != nil {
		return err
	}
	protectBranch.WhitelistUserIDs = whitelist

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

	// Make sure protectBranch.ID is not 0 for whitelists
	if protectBranch.ID == 0 {
		if _, err = db.GetEngine(ctx).Insert(protectBranch); err != nil {
			return fmt.Errorf("Insert: %v", err)
		}
		return nil
	}

	if _, err = db.GetEngine(ctx).ID(protectBranch.ID).AllCols().Update(protectBranch); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	return nil
}

// GetProtectedBranches get all protected branches
func GetProtectedBranches(repoID int64) ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, db.GetEngine(db.DefaultContext).Find(&protectedBranches, &ProtectedBranch{RepoID: repoID})
}

// IsProtectedBranch checks if branch is protected
func IsProtectedBranch(repoID int64, branchName string) (bool, error) {
	protectedBranch := &ProtectedBranch{
		RepoID:     repoID,
		BranchName: branchName,
	}

	has, err := db.GetEngine(db.DefaultContext).Exist(protectedBranch)
	if err != nil {
		return true, err
	}
	return has, nil
}

// updateApprovalWhitelist checks whether the user whitelist changed and returns a whitelist with
// the users from newWhitelist which have explicit read or write access to the repo.
func updateApprovalWhitelist(ctx context.Context, repo *repo_model.Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasUsersChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
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

	return
}

// updateUserWhitelist checks whether the user whitelist changed and returns a whitelist with
// the users from newWhitelist which have write access to the repo.
func updateUserWhitelist(ctx context.Context, repo *repo_model.Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasUsersChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
	if !hasUsersChanged {
		return currentWhitelist, nil
	}

	whitelist = make([]int64, 0, len(newWhitelist))
	for _, userID := range newWhitelist {
		user, err := user_model.GetUserByIDCtx(ctx, userID)
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

	return
}

// updateTeamWhitelist checks whether the team whitelist changed and returns a whitelist with
// the teams from newWhitelist which have write access to the repo.
func updateTeamWhitelist(ctx context.Context, repo *repo_model.Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasTeamsChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
	if !hasTeamsChanged {
		return currentWhitelist, nil
	}

	teams, err := organization.GetTeamsWithAccessToRepo(ctx, repo.OwnerID, repo.ID, perm.AccessModeRead)
	if err != nil {
		return nil, fmt.Errorf("GetTeamsWithAccessToRepo [org_id: %d, repo_id: %d]: %v", repo.OwnerID, repo.ID, err)
	}

	whitelist = make([]int64, 0, len(teams))
	for i := range teams {
		if util.IsInt64InSlice(teams[i].ID, newWhitelist) {
			whitelist = append(whitelist, teams[i].ID)
		}
	}

	return
}

// DeleteProtectedBranch removes ProtectedBranch relation between the user and repository.
func DeleteProtectedBranch(repoID, id int64) (err error) {
	protectedBranch := &ProtectedBranch{
		RepoID: repoID,
		ID:     id,
	}

	if affected, err := db.GetEngine(db.DefaultContext).Delete(protectedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("delete protected branch ID(%v) failed", id)
	}

	return nil
}

// DeletedBranch struct
type DeletedBranch struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string             `xorm:"UNIQUE(s) NOT NULL"`
	Commit      string             `xorm:"UNIQUE(s) NOT NULL"`
	DeletedByID int64              `xorm:"INDEX"`
	DeletedBy   *user_model.User   `xorm:"-"`
	DeletedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

// AddDeletedBranch adds a deleted branch to the database
func AddDeletedBranch(repoID int64, branchName, commit string, deletedByID int64) error {
	deletedBranch := &DeletedBranch{
		RepoID:      repoID,
		Name:        branchName,
		Commit:      commit,
		DeletedByID: deletedByID,
	}

	_, err := db.GetEngine(db.DefaultContext).Insert(deletedBranch)
	return err
}

// GetDeletedBranches returns all the deleted branches
func GetDeletedBranches(repoID int64) ([]*DeletedBranch, error) {
	deletedBranches := make([]*DeletedBranch, 0)
	return deletedBranches, db.GetEngine(db.DefaultContext).Where("repo_id = ?", repoID).Desc("deleted_unix").Find(&deletedBranches)
}

// GetDeletedBranchByID get a deleted branch by its ID
func GetDeletedBranchByID(repoID, id int64) (*DeletedBranch, error) {
	deletedBranch := &DeletedBranch{}
	has, err := db.GetEngine(db.DefaultContext).Where("repo_id = ?", repoID).And("id = ?", id).Get(deletedBranch)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return deletedBranch, nil
}

// RemoveDeletedBranchByID removes a deleted branch from the database
func RemoveDeletedBranchByID(repoID, id int64) (err error) {
	deletedBranch := &DeletedBranch{
		RepoID: repoID,
		ID:     id,
	}

	if affected, err := db.GetEngine(db.DefaultContext).Delete(deletedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("remove deleted branch ID(%v) failed", id)
	}

	return nil
}

// LoadUser loads the user that deleted the branch
// When there's no user found it returns a user_model.NewGhostUser
func (deletedBranch *DeletedBranch) LoadUser() {
	user, err := user_model.GetUserByID(deletedBranch.DeletedByID)
	if err != nil {
		user = user_model.NewGhostUser()
	}
	deletedBranch.DeletedBy = user
}

// RemoveDeletedBranchByName removes all deleted branches
func RemoveDeletedBranchByName(repoID int64, branch string) error {
	_, err := db.GetEngine(db.DefaultContext).Where("repo_id=? AND name=?", repoID, branch).Delete(new(DeletedBranch))
	return err
}

// RemoveOldDeletedBranches removes old deleted branches
func RemoveOldDeletedBranches(ctx context.Context, olderThan time.Duration) {
	// Nothing to do for shutdown or terminate
	log.Trace("Doing: DeletedBranchesCleanup")

	deleteBefore := time.Now().Add(-olderThan)
	_, err := db.GetEngine(db.DefaultContext).Where("deleted_unix < ?", deleteBefore.Unix()).Delete(new(DeletedBranch))
	if err != nil {
		log.Error("DeletedBranchesCleanup: %v", err)
	}
}

// RenamedBranch provide renamed branch log
// will check it when a branch can't be found
type RenamedBranch struct {
	ID          int64 `xorm:"pk autoincr"`
	RepoID      int64 `xorm:"INDEX NOT NULL"`
	From        string
	To          string
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// FindRenamedBranch check if a branch was renamed
func FindRenamedBranch(repoID int64, from string) (branch *RenamedBranch, exist bool, err error) {
	branch = &RenamedBranch{
		RepoID: repoID,
		From:   from,
	}
	exist, err = db.GetEngine(db.DefaultContext).Get(branch)

	return
}

// RenameBranch rename a branch
func RenameBranch(repo *repo_model.Repository, from, to string, gitAction func(isDefault bool) error) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)
	// 1. update default branch if needed
	isDefault := repo.DefaultBranch == from
	if isDefault {
		repo.DefaultBranch = to
		_, err = sess.ID(repo.ID).Cols("default_branch").Update(repo)
		if err != nil {
			return err
		}
	}

	// 2. Update protected branch if needed
	protectedBranch, err := getProtectedBranchBy(sess, repo.ID, from)
	if err != nil {
		return err
	}

	if protectedBranch != nil {
		protectedBranch.BranchName = to
		_, err = sess.ID(protectedBranch.ID).Cols("branch_name").Update(protectedBranch)
		if err != nil {
			return err
		}
	}

	// 3. Update all not merged pull request base branch name
	_, err = sess.Table(new(PullRequest)).Where("base_repo_id=? AND base_branch=? AND has_merged=?",
		repo.ID, from, false).
		Update(map[string]interface{}{"base_branch": to})
	if err != nil {
		return err
	}

	// 4. do git action
	if err = gitAction(isDefault); err != nil {
		return err
	}

	// 5. insert renamed branch record
	renamedBranch := &RenamedBranch{
		RepoID: repo.ID,
		From:   from,
		To:     to,
	}
	err = db.Insert(ctx, renamedBranch)
	if err != nil {
		return err
	}

	return committer.Commit()
}
