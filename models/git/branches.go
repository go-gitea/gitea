// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

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

func init() {
	db.RegisterModel(new(DeletedBranch))
	db.RegisterModel(new(RenamedBranch))
}

// AddDeletedBranch adds a deleted branch to the database
func AddDeletedBranch(ctx context.Context, repoID int64, branchName, commit string, deletedByID int64) error {
	deletedBranch := &DeletedBranch{
		RepoID:      repoID,
		Name:        branchName,
		Commit:      commit,
		DeletedByID: deletedByID,
	}

	_, err := db.GetEngine(ctx).Insert(deletedBranch)
	return err
}

// GetDeletedBranches returns all the deleted branches
func GetDeletedBranches(ctx context.Context, repoID int64) ([]*DeletedBranch, error) {
	deletedBranches := make([]*DeletedBranch, 0)
	return deletedBranches, db.GetEngine(ctx).Where("repo_id = ?", repoID).Desc("deleted_unix").Find(&deletedBranches)
}

// GetDeletedBranchByID get a deleted branch by its ID
func GetDeletedBranchByID(ctx context.Context, repoID, id int64) (*DeletedBranch, error) {
	deletedBranch := &DeletedBranch{}
	has, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).And("id = ?", id).Get(deletedBranch)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return deletedBranch, nil
}

// RemoveDeletedBranchByID removes a deleted branch from the database
func RemoveDeletedBranchByID(ctx context.Context, repoID, id int64) (err error) {
	deletedBranch := &DeletedBranch{
		RepoID: repoID,
		ID:     id,
	}

	if affected, err := db.GetEngine(ctx).Delete(deletedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("remove deleted branch ID(%v) failed", id)
	}

	return nil
}

// LoadUser loads the user that deleted the branch
// When there's no user found it returns a user_model.NewGhostUser
func (deletedBranch *DeletedBranch) LoadUser(ctx context.Context) {
	user, err := user_model.GetUserByID(ctx, deletedBranch.DeletedByID)
	if err != nil {
		user = user_model.NewGhostUser()
	}
	deletedBranch.DeletedBy = user
}

// RemoveDeletedBranchByName removes all deleted branches
func RemoveDeletedBranchByName(ctx context.Context, repoID int64, branch string) error {
	_, err := db.GetEngine(ctx).Where("repo_id=? AND name=?", repoID, branch).Delete(new(DeletedBranch))
	return err
}

// RemoveOldDeletedBranches removes old deleted branches
func RemoveOldDeletedBranches(ctx context.Context, olderThan time.Duration) {
	// Nothing to do for shutdown or terminate
	log.Trace("Doing: DeletedBranchesCleanup")

	deleteBefore := time.Now().Add(-olderThan)
	_, err := db.GetEngine(ctx).Where("deleted_unix < ?", deleteBefore.Unix()).Delete(new(DeletedBranch))
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
func FindRenamedBranch(ctx context.Context, repoID int64, from string) (branch *RenamedBranch, exist bool, err error) {
	branch = &RenamedBranch{
		RepoID: repoID,
		From:   from,
	}
	exist, err = db.GetEngine(ctx).Get(branch)

	return branch, exist, err
}

// RenameBranch rename a branch
func RenameBranch(ctx context.Context, repo *repo_model.Repository, from, to string, gitAction func(isDefault bool) error) (err error) {
	ctx, committer, err := db.TxContext(ctx)
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
	protectedBranch, err := GetProtectedBranchRuleByName(ctx, repo.ID, from)
	if err != nil {
		return err
	}

	if protectedBranch != nil {
		protectedBranch.RuleName = to
		_, err = sess.ID(protectedBranch.ID).Cols("branch_name").Update(protectedBranch)
		if err != nil {
			return err
		}
	} else {
		protected, err := IsBranchProtected(ctx, repo.ID, from)
		if err != nil {
			return err
		}
		if protected {
			return ErrBranchIsProtected
		}
	}

	// 3. Update all not merged pull request base branch name
	_, err = sess.Table("pull_request").Where("base_repo_id=? AND base_branch=? AND has_merged=?",
		repo.ID, from, false).
		Update(map[string]any{"base_branch": to})
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
