// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// Branch represents a branch of a repository
// For those repository who have many branches, stored into database is a good choice
// for pagination, keyword search and filtering
type Branch struct {
	ID            int64
	RepoID        int64  `xorm:"index UNIQUE(s)"`
	Name          string `xorm:"UNIQUE(s) NOT NULL"`
	CommitSHA     string
	CommitMessage string `xorm:"TEXT"`
	PusherID      int64
	IsDeleted     bool
	DeletedByID   int64
	DeletedUnix   timeutil.TimeStamp
	CommitTime    timeutil.TimeStamp // The commit
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(Branch))
	db.RegisterModel(new(RenamedBranch))
}

func LoadAllBranches(ctx context.Context, repoID int64) ([]*Branch, error) {
	var branches []*Branch
	err := db.GetEngine(ctx).Where("repo_id=?", repoID).Find(&branches)
	return branches, err
}

func GetDefaultBranch(ctx context.Context, repo *repo_model.Repository) (*Branch, error) {
	var branch Branch
	has, err := db.GetEngine(ctx).Where("repo_id=?", repo.ID).And("name=?", repo.DefaultBranch).Get(&branch)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, git.ErrBranchNotExist{Name: repo.DefaultBranch}
	}
	return &branch, nil
}

type FindBranchOptions struct {
	db.ListOptions
	RepoID               int64
	IncludeDefaultBranch bool
	IncludeDeletedBranch bool
}

func FindBranches(ctx context.Context, opts FindBranchOptions) ([]*Branch, int64, error) {
	sess := db.GetEngine(ctx).Where("repo_id=?", opts.RepoID)
	if opts.PageSize > 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	if !opts.IncludeDefaultBranch {
		sess = sess.And(builder.Neq{"name": builder.Select("default_branch").From("repository").Where(builder.Eq{"id": opts.RepoID})})
	}
	if !opts.IncludeDeletedBranch {
		sess.And(builder.Eq{"is_deleted": false})
	}
	var branches []*Branch
	total, err := sess.FindAndCount(&branches)
	if err != nil {
		return nil, 0, err
	}
	return branches, total, err
}

func AddBranch(ctx context.Context, branch *Branch) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Insert(branch); err != nil {
			return err
		}

		return removeDeletedBranchByName(ctx, branch.RepoID, branch.Name)
	})
}

func AddBranches(ctx context.Context, branches []*Branch) error {
	for _, branch := range branches {
		if err := AddBranch(ctx, branch); err != nil {
			return err
		}
	}
	return nil
}

func GetDeletedBranchByID(ctx context.Context, repoID, branchID int64) (*Branch, error) {
	var branch Branch
	has, err := db.GetEngine(ctx).ID(branchID).Get(&branch)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, git.ErrNotExist{}
	}
	if branch.RepoID != repoID {
		return nil, git.ErrNotExist{}
	}
	if !branch.IsDeleted {
		return nil, git.ErrNotExist{}
	}
	return &branch, nil
}

func DeleteBranches(ctx context.Context, repoID, doerID int64, branchIDs []int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		branches := make([]*Branch, 0, len(branchIDs))
		if err := db.GetEngine(ctx).In("id", branchIDs).Find(&branches); err != nil {
			return err
		}
		for _, branch := range branches {
			if err := AddDeletedBranch(ctx, repoID, branch.Name, doerID); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateBranch updates the branch information in the database. If the branch exist, it will update latest commit of this branch information
// If it doest not exist, insert a new record into database
func UpdateBranch(ctx context.Context, repoID int64, branchName, commitID, commitMessage string, pusherID int64, commitTime time.Time) error {
	if err := removeDeletedBranchByName(ctx, repoID, branchName); err != nil {
		return err
	}
	cnt, err := db.GetEngine(ctx).Where("repo_id=? AND name=?", repoID, branchName).
		Cols("commit_sha, commit_message, pusher_id, commit_time, updated_unix").
		Update(&Branch{
			CommitSHA:     commitID,
			CommitMessage: commitMessage,
			PusherID:      pusherID,
			CommitTime:    timeutil.TimeStamp(commitTime.Unix()),
		})
	if err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}

	return db.Insert(ctx, &Branch{
		RepoID:        repoID,
		Name:          branchName,
		CommitSHA:     commitID,
		CommitMessage: commitMessage,
		PusherID:      pusherID,
		CommitTime:    timeutil.TimeStamp(commitTime.Unix()),
	})
}

// AddDeletedBranch adds a deleted branch to the database
func AddDeletedBranch(ctx context.Context, repoID int64, branchName string, deletedByID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id=? AND name=?", repoID, branchName).
		Cols("is_deleted, deleted_by_id, deleted_unix").
		Update(&Branch{
			IsDeleted:   true,
			DeletedByID: deletedByID,
			DeletedUnix: timeutil.TimeStampNow(),
		})
	return err
}

// removeDeletedBranchByName removes all deleted branches
func removeDeletedBranchByName(ctx context.Context, repoID int64, branch string) error {
	_, err := db.GetEngine(ctx).Where("repo_id=? AND name=? AND is_deleted = ?", repoID, branch, true).Delete(new(Branch))
	return err
}

// RemoveOldDeletedBranches removes old deleted branches
func RemoveOldDeletedBranches(ctx context.Context, olderThan time.Duration) {
	// Nothing to do for shutdown or terminate
	log.Trace("Doing: DeletedBranchesCleanup")

	deleteBefore := time.Now().Add(-olderThan)
	_, err := db.GetEngine(ctx).Where("deleted_unix < ?", deleteBefore.Unix()).Delete(new(Branch))
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

	// 1. update branch in database
	if n, err := sess.Where("repo_id=? AND name=?", repo.ID, from).Update(&Branch{
		Name: to,
	}); err != nil {
		return err
	} else if n <= 0 {
		// branch does not exist in the database, so we think branch is not existed
		return nil
	}

	// 2. update default branch if needed
	isDefault := repo.DefaultBranch == from
	if isDefault {
		repo.DefaultBranch = to
		_, err = sess.ID(repo.ID).Cols("default_branch").Update(repo)
		if err != nil {
			return err
		}
	}

	// 3. Update protected branch if needed
	protectedBranch, err := GetProtectedBranchRuleByName(ctx, repo.ID, from)
	if err != nil {
		return err
	}

	if protectedBranch != nil {
		// there is a protect rule for this branch
		protectedBranch.RuleName = to
		_, err = sess.ID(protectedBranch.ID).Cols("branch_name").Update(protectedBranch)
		if err != nil {
			return err
		}
	} else {
		// some glob protect rules may match this branch
		protected, err := IsBranchProtected(ctx, repo.ID, from)
		if err != nil {
			return err
		}
		if protected {
			return ErrBranchIsProtected
		}
	}

	// 4. Update all not merged pull request base branch name
	_, err = sess.Table("pull_request").Where("base_repo_id=? AND base_branch=? AND has_merged=?",
		repo.ID, from, false).
		Update(map[string]interface{}{"base_branch": to})
	if err != nil {
		return err
	}

	// 5. do git action
	if err = gitAction(isDefault); err != nil {
		return err
	}

	// 6. insert renamed branch record
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
