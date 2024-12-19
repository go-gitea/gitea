// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrBranchNotExist represents an error that branch with such name does not exist.
type ErrBranchNotExist struct {
	RepoID     int64
	BranchName string
}

// IsErrBranchNotExist checks if an error is an ErrBranchDoesNotExist.
func IsErrBranchNotExist(err error) bool {
	_, ok := err.(ErrBranchNotExist)
	return ok
}

func (err ErrBranchNotExist) Error() string {
	return fmt.Sprintf("branch does not exist [repo_id: %d name: %s]", err.RepoID, err.BranchName)
}

func (err ErrBranchNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrBranchAlreadyExists represents an error that branch with such name already exists.
type ErrBranchAlreadyExists struct {
	BranchName string
}

// IsErrBranchAlreadyExists checks if an error is an ErrBranchAlreadyExists.
func IsErrBranchAlreadyExists(err error) bool {
	_, ok := err.(ErrBranchAlreadyExists)
	return ok
}

func (err ErrBranchAlreadyExists) Error() string {
	return fmt.Sprintf("branch already exists [name: %s]", err.BranchName)
}

func (err ErrBranchAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrBranchNameConflict represents an error that branch name conflicts with other branch.
type ErrBranchNameConflict struct {
	BranchName string
}

// IsErrBranchNameConflict checks if an error is an ErrBranchNameConflict.
func IsErrBranchNameConflict(err error) bool {
	_, ok := err.(ErrBranchNameConflict)
	return ok
}

func (err ErrBranchNameConflict) Error() string {
	return fmt.Sprintf("branch conflicts with existing branch [name: %s]", err.BranchName)
}

func (err ErrBranchNameConflict) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrBranchesEqual represents an error that base branch is equal to the head branch.
type ErrBranchesEqual struct {
	BaseBranchName string
	HeadBranchName string
}

// IsErrBranchesEqual checks if an error is an ErrBranchesEqual.
func IsErrBranchesEqual(err error) bool {
	_, ok := err.(ErrBranchesEqual)
	return ok
}

func (err ErrBranchesEqual) Error() string {
	return fmt.Sprintf("branches are equal [head: %sm base: %s]", err.HeadBranchName, err.BaseBranchName)
}

func (err ErrBranchesEqual) Unwrap() error {
	return util.ErrInvalidArgument
}

// Branch represents a branch of a repository
// For those repository who have many branches, stored into database is a good choice
// for pagination, keyword search and filtering
type Branch struct {
	ID            int64
	RepoID        int64                  `xorm:"UNIQUE(s)"`
	Repo          *repo_model.Repository `xorm:"-"`
	Name          string                 `xorm:"UNIQUE(s) NOT NULL"` // git's ref-name is case-sensitive internally, however, in some databases (mssql, mysql, by default), it's case-insensitive at the moment
	CommitID      string
	CommitMessage string `xorm:"TEXT"` // it only stores the message summary (the first line)
	PusherID      int64
	Pusher        *user_model.User `xorm:"-"`
	IsDeleted     bool             `xorm:"index"`
	DeletedByID   int64
	DeletedBy     *user_model.User   `xorm:"-"`
	DeletedUnix   timeutil.TimeStamp `xorm:"index"`
	CommitTime    timeutil.TimeStamp // The commit
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

func (b *Branch) LoadDeletedBy(ctx context.Context) (err error) {
	if b.DeletedBy == nil {
		b.DeletedBy, err = user_model.GetUserByID(ctx, b.DeletedByID)
		if user_model.IsErrUserNotExist(err) {
			b.DeletedBy = user_model.NewGhostUser()
			err = nil
		}
	}
	return err
}

func (b *Branch) LoadPusher(ctx context.Context) (err error) {
	if b.Pusher == nil && b.PusherID > 0 {
		b.Pusher, err = user_model.GetUserByID(ctx, b.PusherID)
		if user_model.IsErrUserNotExist(err) {
			b.Pusher = user_model.NewGhostUser()
			err = nil
		}
	}
	return err
}

func (b *Branch) LoadRepo(ctx context.Context) (err error) {
	if b.Repo != nil || b.RepoID == 0 {
		return nil
	}
	b.Repo, err = repo_model.GetRepositoryByID(ctx, b.RepoID)
	return err
}

func init() {
	db.RegisterModel(new(Branch))
	db.RegisterModel(new(RenamedBranch))
}

func GetBranch(ctx context.Context, repoID int64, branchName string) (*Branch, error) {
	var branch Branch
	has, err := db.GetEngine(ctx).Where("repo_id=?", repoID).And("name=?", branchName).Get(&branch)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrBranchNotExist{
			RepoID:     repoID,
			BranchName: branchName,
		}
	}
	return &branch, nil
}

func GetBranches(ctx context.Context, repoID int64, branchNames []string, includeDeleted bool) ([]*Branch, error) {
	branches := make([]*Branch, 0, len(branchNames))

	sess := db.GetEngine(ctx).Where("repo_id=?", repoID).In("name", branchNames)
	if !includeDeleted {
		sess.And("is_deleted=?", false)
	}
	return branches, sess.Find(&branches)
}

func BranchesToNamesSet(branches []*Branch) container.Set[string] {
	names := make(container.Set[string], len(branches))
	for _, branch := range branches {
		names.Add(branch.Name)
	}
	return names
}

func AddBranches(ctx context.Context, branches []*Branch) error {
	for _, branch := range branches {
		if _, err := db.GetEngine(ctx).Insert(branch); err != nil {
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
		return nil, ErrBranchNotExist{
			RepoID: repoID,
		}
	}
	if branch.RepoID != repoID {
		return nil, ErrBranchNotExist{
			RepoID: repoID,
		}
	}
	if !branch.IsDeleted {
		return nil, ErrBranchNotExist{
			RepoID: repoID,
		}
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

// UpdateBranch updates the branch information in the database.
func UpdateBranch(ctx context.Context, repoID, pusherID int64, branchName string, commit *git.Commit) (int64, error) {
	return db.GetEngine(ctx).Where("repo_id=? AND name=?", repoID, branchName).
		Cols("commit_id, commit_message, pusher_id, commit_time, is_deleted, updated_unix").
		Update(&Branch{
			CommitID:      commit.ID.String(),
			CommitMessage: commit.Summary(),
			PusherID:      pusherID,
			CommitTime:    timeutil.TimeStamp(commit.Committer.When.Unix()),
			IsDeleted:     false,
		})
}

// AddDeletedBranch adds a deleted branch to the database
func AddDeletedBranch(ctx context.Context, repoID int64, branchName string, deletedByID int64) error {
	branch, err := GetBranch(ctx, repoID, branchName)
	if err != nil {
		return err
	}
	if branch.IsDeleted {
		return nil
	}

	cnt, err := db.GetEngine(ctx).Where("repo_id=? AND name=? AND is_deleted=?", repoID, branchName, false).
		Cols("is_deleted, deleted_by_id, deleted_unix").
		Update(&Branch{
			IsDeleted:   true,
			DeletedByID: deletedByID,
			DeletedUnix: timeutil.TimeStampNow(),
		})
	if err != nil {
		return err
	}
	if cnt == 0 {
		return fmt.Errorf("branch %s not found or has been deleted", branchName)
	}
	return err
}

func RemoveDeletedBranchByID(ctx context.Context, repoID, branchID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id=? AND id=? AND is_deleted = ?", repoID, branchID, true).Delete(new(Branch))
	return err
}

// RemoveOldDeletedBranches removes old deleted branches
func RemoveOldDeletedBranches(ctx context.Context, olderThan time.Duration) {
	// Nothing to do for shutdown or terminate
	log.Trace("Doing: DeletedBranchesCleanup")

	deleteBefore := time.Now().Add(-olderThan)
	_, err := db.GetEngine(ctx).Where("is_deleted=? AND deleted_unix < ?", true, deleteBefore.Unix()).Delete(new(Branch))
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
func RenameBranch(ctx context.Context, repo *repo_model.Repository, from, to string, gitAction func(ctx context.Context, isDefault bool) error) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	// check whether from branch exist
	var branch Branch
	exist, err := db.GetEngine(ctx).Where("repo_id=? AND name=?", repo.ID, from).Get(&branch)
	if err != nil {
		return err
	} else if !exist || branch.IsDeleted {
		return ErrBranchNotExist{
			RepoID:     repo.ID,
			BranchName: from,
		}
	}

	// check whether to branch exist or is_deleted
	var dstBranch Branch
	exist, err = db.GetEngine(ctx).Where("repo_id=? AND name=?", repo.ID, to).Get(&dstBranch)
	if err != nil {
		return err
	}
	if exist {
		if !dstBranch.IsDeleted {
			return ErrBranchAlreadyExists{
				BranchName: to,
			}
		}

		if _, err := db.GetEngine(ctx).ID(dstBranch.ID).NoAutoCondition().Delete(&dstBranch); err != nil {
			return err
		}
	}

	// 1. update branch in database
	if n, err := sess.Where("repo_id=? AND name=?", repo.ID, from).Update(&Branch{
		Name: to,
	}); err != nil {
		return err
	} else if n <= 0 {
		return ErrBranchNotExist{
			RepoID:     repo.ID,
			BranchName: from,
		}
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
		Update(map[string]any{"base_branch": to})
	if err != nil {
		return err
	}

	// 4.1 Update all not merged pull request head branch name
	if _, err = sess.Table("pull_request").Where("head_repo_id=? AND head_branch=? AND has_merged=?",
		repo.ID, from, false).
		Update(map[string]any{"head_branch": to}); err != nil {
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

	// 6. do git action
	if err = gitAction(ctx, isDefault); err != nil {
		return err
	}

	return committer.Commit()
}

type FindRecentlyPushedNewBranchesOptions struct {
	Repo            *repo_model.Repository
	BaseRepo        *repo_model.Repository
	CommitAfterUnix int64
	MaxCount        int
}

type RecentlyPushedNewBranch struct {
	BranchDisplayName string
	BranchLink        string
	BranchCompareURL  string
	CommitTime        timeutil.TimeStamp
}

// FindRecentlyPushedNewBranches return at most 2 new branches pushed by the user in 2 hours which has no opened PRs created
// if opts.CommitAfterUnix is 0, we will find the branches that were committed to in the last 2 hours
// if opts.ListOptions is not set, we will only display top 2 latest branches.
// Protected branches will be skipped since they are unlikely to be used to create new PRs.
func FindRecentlyPushedNewBranches(ctx context.Context, doer *user_model.User, opts *FindRecentlyPushedNewBranchesOptions) ([]*RecentlyPushedNewBranch, error) {
	if doer == nil {
		return []*RecentlyPushedNewBranch{}, nil
	}

	// find all related repo ids
	repoOpts := repo_model.SearchRepoOptions{
		Actor:      doer,
		Private:    true,
		AllPublic:  false, // Include also all public repositories of users and public organisations
		AllLimited: false, // Include also all public repositories of limited organisations
		Fork:       optional.Some(true),
		ForkFrom:   opts.BaseRepo.ID,
		Archived:   optional.Some(false),
	}
	repoCond := repo_model.SearchRepositoryCondition(&repoOpts).And(repo_model.AccessibleRepositoryCondition(doer, unit.TypeCode))
	if opts.Repo.ID == opts.BaseRepo.ID {
		// should also include the base repo's branches
		repoCond = repoCond.Or(builder.Eq{"id": opts.BaseRepo.ID})
	} else {
		// in fork repo, we only detect the fork repo's branch
		repoCond = repoCond.And(builder.Eq{"id": opts.Repo.ID})
	}
	repoIDs := builder.Select("id").From("repository").Where(repoCond)

	if opts.CommitAfterUnix == 0 {
		opts.CommitAfterUnix = time.Now().Add(-time.Hour * 2).Unix()
	}

	baseBranch, err := GetBranch(ctx, opts.BaseRepo.ID, opts.BaseRepo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	// find all related branches, these branches may already created PRs, we will check later
	var branches []*Branch
	if err := db.GetEngine(ctx).
		Where(builder.And(
			builder.Eq{
				"pusher_id":  doer.ID,
				"is_deleted": false,
			},
			builder.Gte{"commit_time": opts.CommitAfterUnix},
			builder.In("repo_id", repoIDs),
			// newly created branch have no changes, so skip them
			builder.Neq{"commit_id": baseBranch.CommitID},
		)).
		OrderBy(db.SearchOrderByRecentUpdated.String()).
		Find(&branches); err != nil {
		return nil, err
	}

	newBranches := make([]*RecentlyPushedNewBranch, 0, len(branches))
	if opts.MaxCount == 0 {
		// by default we display 2 recently pushed new branch
		opts.MaxCount = 2
	}
	for _, branch := range branches {
		// whether the branch is protected
		protected, err := IsBranchProtected(ctx, branch.RepoID, branch.Name)
		if err != nil {
			return nil, fmt.Errorf("IsBranchProtected: %v", err)
		}
		if protected {
			// Skip protected branches,
			// since updates to protected branches often come from PR merges,
			// and they are unlikely to be used to create new PRs.
			continue
		}

		// whether branch have already created PR
		count, err := db.GetEngine(ctx).Table("pull_request").
			// we should not only use branch name here, because if there are branches with same name in other repos,
			// we can not detect them correctly
			Where(builder.Eq{"head_repo_id": branch.RepoID, "head_branch": branch.Name}).Count()
		if err != nil {
			return nil, err
		}

		// if no PR, we add to the result
		if count == 0 {
			if err := branch.LoadRepo(ctx); err != nil {
				return nil, err
			}

			branchDisplayName := branch.Name
			if branch.Repo.ID != opts.BaseRepo.ID && branch.Repo.ID != opts.Repo.ID {
				branchDisplayName = fmt.Sprintf("%s:%s", branch.Repo.FullName(), branchDisplayName)
			}
			newBranches = append(newBranches, &RecentlyPushedNewBranch{
				BranchDisplayName: branchDisplayName,
				BranchLink:        fmt.Sprintf("%s/src/branch/%s", branch.Repo.Link(), util.PathEscapeSegments(branch.Name)),
				BranchCompareURL:  branch.Repo.ComposeBranchCompareURL(opts.BaseRepo, branch.Name),
				CommitTime:        branch.CommitTime,
			})
		}
		if len(newBranches) == opts.MaxCount {
			break
		}
	}

	return newBranches, nil
}
