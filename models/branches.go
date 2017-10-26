// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
)

const (
	// ProtectedBranchRepoID protected Repo ID
	ProtectedBranchRepoID = "GITEA_REPO_ID"
)

// ProtectedBranch struct
type ProtectedBranch struct {
	ID               int64  `xorm:"pk autoincr"`
	RepoID           int64  `xorm:"UNIQUE(s)"`
	BranchName       string `xorm:"UNIQUE(s)"`
	CanPush          bool   `xorm:"NOT NULL DEFAULT false"`
	EnableWhitelist  bool
	WhitelistUserIDs []int64   `xorm:"JSON TEXT"`
	WhitelistTeamIDs []int64   `xorm:"JSON TEXT"`
	Created          time.Time `xorm:"-"`
	CreatedUnix      int64     `xorm:"created"`
	Updated          time.Time `xorm:"-"`
	UpdatedUnix      int64     `xorm:"updated"`
}

// IsProtected returns if the branch is protected
func (protectBranch *ProtectedBranch) IsProtected() bool {
	return protectBranch.ID > 0
}

// CanUserPush returns if some user could push to this protected branch
func (protectBranch *ProtectedBranch) CanUserPush(userID int64) bool {
	if !protectBranch.EnableWhitelist {
		return false
	}

	if base.Int64sContains(protectBranch.WhitelistUserIDs, userID) {
		return true
	}

	if len(protectBranch.WhitelistTeamIDs) == 0 {
		return false
	}

	in, err := IsUserInTeams(userID, protectBranch.WhitelistTeamIDs)
	if err != nil {
		log.Error(1, "IsUserInTeams:", err)
		return false
	}
	return in
}

// GetProtectedBranchByRepoID getting protected branch by repo ID
func GetProtectedBranchByRepoID(RepoID int64) ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, x.Where("repo_id = ?", RepoID).Desc("updated_unix").Find(&protectedBranches)
}

// GetProtectedBranchBy getting protected branch by ID/Name
func GetProtectedBranchBy(repoID int64, BranchName string) (*ProtectedBranch, error) {
	rel := &ProtectedBranch{RepoID: repoID, BranchName: strings.ToLower(BranchName)}
	has, err := x.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rel, nil
}

// GetProtectedBranchByID getting protected branch by ID
func GetProtectedBranchByID(id int64) (*ProtectedBranch, error) {
	rel := &ProtectedBranch{ID: id}
	has, err := x.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rel, nil
}

// UpdateProtectBranch saves branch protection options of repository.
// If ID is 0, it creates a new record. Otherwise, updates existing record.
// This function also performs check if whitelist user and team's IDs have been changed
// to avoid unnecessary whitelist delete and regenerate.
func UpdateProtectBranch(repo *Repository, protectBranch *ProtectedBranch, whitelistUserIDs, whitelistTeamIDs []int64) (err error) {
	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}

	hasUsersChanged := !util.IsSliceInt64Eq(protectBranch.WhitelistUserIDs, whitelistUserIDs)
	if hasUsersChanged {
		protectBranch.WhitelistUserIDs = make([]int64, 0, len(whitelistUserIDs))
		for _, userID := range whitelistUserIDs {
			has, err := hasAccess(x, userID, repo, AccessModeWrite)
			if err != nil {
				return fmt.Errorf("HasAccess [user_id: %d, repo_id: %d]: %v", userID, protectBranch.RepoID, err)
			} else if !has {
				continue // Drop invalid user ID
			}

			protectBranch.WhitelistUserIDs = append(protectBranch.WhitelistUserIDs, userID)
		}
	}

	// if the repo is in an orgniziation
	hasTeamsChanged := !util.IsSliceInt64Eq(protectBranch.WhitelistTeamIDs, whitelistTeamIDs)
	if hasTeamsChanged {
		teams, err := GetTeamsWithAccessToRepo(repo.OwnerID, repo.ID, AccessModeWrite)
		if err != nil {
			return fmt.Errorf("GetTeamsWithAccessToRepo [org_id: %d, repo_id: %d]: %v", repo.OwnerID, repo.ID, err)
		}
		protectBranch.WhitelistTeamIDs = make([]int64, 0, len(teams))
		for i := range teams {
			if teams[i].HasWriteAccess() && com.IsSliceContainsInt64(whitelistTeamIDs, teams[i].ID) {
				protectBranch.WhitelistTeamIDs = append(protectBranch.WhitelistTeamIDs, teams[i].ID)
			}
		}
	}

	// Make sure protectBranch.ID is not 0 for whitelists
	if protectBranch.ID == 0 {
		if _, err = x.Insert(protectBranch); err != nil {
			return fmt.Errorf("Insert: %v", err)
		}
		return nil
	}

	if _, err = x.ID(protectBranch.ID).AllCols().Update(protectBranch); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	return nil
}

// GetProtectedBranches get all protected branches
func (repo *Repository) GetProtectedBranches() ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, x.Find(&protectedBranches, &ProtectedBranch{RepoID: repo.ID})
}

// IsProtectedBranch checks if branch is protected
func (repo *Repository) IsProtectedBranch(branchName string, doer *User) (bool, error) {
	protectedBranch := &ProtectedBranch{
		RepoID:     repo.ID,
		BranchName: branchName,
	}

	has, err := x.Get(protectedBranch)
	if err != nil {
		return true, err
	} else if has {
		return !protectedBranch.CanUserPush(doer.ID), nil
	}

	return false, nil
}

// DeleteProtectedBranch removes ProtectedBranch relation between the user and repository.
func (repo *Repository) DeleteProtectedBranch(id int64) (err error) {
	protectedBranch := &ProtectedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if affected, err := sess.Delete(protectedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("delete protected branch ID(%v) failed", id)
	}

	return sess.Commit()
}

// DeletedBranch struct
type DeletedBranch struct {
	ID          int64     `xorm:"pk autoincr"`
	RepoID      int64     `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string    `xorm:"UNIQUE(s) NOT NULL"`
	Commit      string    `xorm:"UNIQUE(s) NOT NULL"`
	DeletedByID int64     `xorm:"INDEX"`
	DeletedBy   *User     `xorm:"-"`
	Deleted     time.Time `xorm:"-"`
	DeletedUnix int64     `xorm:"INDEX created"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (deletedBranch *DeletedBranch) AfterLoad() {
	deletedBranch.Deleted = time.Unix(deletedBranch.DeletedUnix, 0).Local()
}

// AddDeletedBranch adds a deleted branch to the database
func (repo *Repository) AddDeletedBranch(branchName, commit string, deletedByID int64) error {
	deletedBranch := &DeletedBranch{
		RepoID:      repo.ID,
		Name:        branchName,
		Commit:      commit,
		DeletedByID: deletedByID,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.InsertOne(deletedBranch); err != nil {
		return err
	}

	return sess.Commit()
}

// GetDeletedBranches returns all the deleted branches
func (repo *Repository) GetDeletedBranches() ([]*DeletedBranch, error) {
	deletedBranches := make([]*DeletedBranch, 0)
	return deletedBranches, x.Where("repo_id = ?", repo.ID).Desc("deleted_unix").Find(&deletedBranches)
}

// GetDeletedBranchByID get a deleted branch by its ID
func (repo *Repository) GetDeletedBranchByID(ID int64) (*DeletedBranch, error) {
	deletedBranch := &DeletedBranch{ID: ID}
	has, err := x.Get(deletedBranch)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return deletedBranch, nil
}

// RemoveDeletedBranch removes a deleted branch from the database
func (repo *Repository) RemoveDeletedBranch(id int64) (err error) {
	deletedBranch := &DeletedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if affected, err := sess.Delete(deletedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("remove deleted branch ID(%v) failed", id)
	}

	return sess.Commit()
}

// LoadUser loads the user that deleted the branch
// When there's no user found it returns a NewGhostUser
func (deletedBranch *DeletedBranch) LoadUser() {
	user, err := GetUserByID(deletedBranch.DeletedByID)
	if err != nil {
		user = NewGhostUser()
	}
	deletedBranch.DeletedBy = user
}

// RemoveOldDeletedBranches removes old deleted branches
func RemoveOldDeletedBranches() {
	if !taskStatusTable.StartIfNotRunning(`deleted_branches_cleanup`) {
		return
	}
	defer taskStatusTable.Stop(`deleted_branches_cleanup`)

	log.Trace("Doing: DeletedBranchesCleanup")

	deleteBefore := time.Now().Add(-setting.Cron.DeletedBranchesCleanup.OlderThan)
	_, err := x.Where("deleted_unix < ?", deleteBefore.Unix()).Delete(new(DeletedBranch))
	if err != nil {
		log.Error(4, "DeletedBranchesCleanup: %v", err)
	}
}
