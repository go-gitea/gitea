// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrNoPendingRepoTransfer is an error type for repositories without a pending
// transfer request
type ErrNoPendingRepoTransfer struct {
	RepoID int64
}

func (err ErrNoPendingRepoTransfer) Error() string {
	return fmt.Sprintf("repository doesn't have a pending transfer [repo_id: %d]", err.RepoID)
}

// IsErrNoPendingTransfer is an error type when a repository has no pending
// transfers
func IsErrNoPendingTransfer(err error) bool {
	_, ok := err.(ErrNoPendingRepoTransfer)
	return ok
}

func (err ErrNoPendingRepoTransfer) Unwrap() error {
	return util.ErrNotExist
}

// ErrRepoTransferInProgress represents the state of a repository that has an
// ongoing transfer
type ErrRepoTransferInProgress struct {
	Uname string
	Name  string
}

// IsErrRepoTransferInProgress checks if an error is a ErrRepoTransferInProgress.
func IsErrRepoTransferInProgress(err error) bool {
	_, ok := err.(ErrRepoTransferInProgress)
	return ok
}

func (err ErrRepoTransferInProgress) Error() string {
	return fmt.Sprintf("repository is already being transferred [uname: %s, name: %s]", err.Uname, err.Name)
}

func (err ErrRepoTransferInProgress) Unwrap() error {
	return util.ErrAlreadyExist
}

// RepoTransfer is used to manage repository transfers
type RepoTransfer struct { //nolint
	ID          int64 `xorm:"pk autoincr"`
	DoerID      int64
	Doer        *user_model.User `xorm:"-"`
	RecipientID int64
	Recipient   *user_model.User `xorm:"-"`
	RepoID      int64
	Repo        *Repository `xorm:"-"`
	TeamIDs     []int64
	Teams       []*organization.Team `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL updated"`
}

func init() {
	db.RegisterModel(new(RepoTransfer))
}

func (r *RepoTransfer) LoadRecipient(ctx context.Context) error {
	if r.Recipient == nil {
		u, err := user_model.GetUserByID(ctx, r.RecipientID)
		if err != nil {
			return err
		}
		r.Recipient = u
	}

	return nil
}

func (r *RepoTransfer) LoadRepo(ctx context.Context) error {
	if r.Repo == nil {
		repo, err := GetRepositoryByID(ctx, r.RepoID)
		if err != nil {
			return err
		}
		r.Repo = repo
	}

	return nil
}

// LoadAttributes fetches the transfer recipient from the database
func (r *RepoTransfer) LoadAttributes(ctx context.Context) error {
	if err := r.LoadRecipient(ctx); err != nil {
		return err
	}

	if r.Recipient.IsOrganization() && r.Teams == nil {
		teamsMap, err := organization.GetTeamsByIDs(ctx, r.TeamIDs)
		if err != nil {
			return err
		}
		for _, team := range teamsMap {
			r.Teams = append(r.Teams, team)
		}
	}

	if err := r.LoadRepo(ctx); err != nil {
		return err
	}

	if r.Doer == nil {
		u, err := user_model.GetUserByID(ctx, r.DoerID)
		if err != nil {
			return err
		}
		r.Doer = u
	}

	return nil
}

// CanUserAcceptOrRejectTransfer checks if the user has the rights to accept/decline a repo transfer.
// For user, it checks if it's himself
// For organizations, it checks if the user is able to create repos
func (r *RepoTransfer) CanUserAcceptOrRejectTransfer(ctx context.Context, u *user_model.User) bool {
	if err := r.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return false
	}

	if !r.Recipient.IsOrganization() {
		return r.RecipientID == u.ID
	}

	allowed, err := organization.CanCreateOrgRepo(ctx, r.RecipientID, u.ID)
	if err != nil {
		log.Error("CanCreateOrgRepo: %v", err)
		return false
	}

	return allowed
}

type PendingRepositoryTransferOptions struct {
	RepoID      int64
	SenderID    int64
	RecipientID int64
}

func (opts *PendingRepositoryTransferOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.SenderID != 0 {
		cond = cond.And(builder.Eq{"doer_id": opts.SenderID})
	}
	if opts.RecipientID != 0 {
		cond = cond.And(builder.Eq{"recipient_id": opts.RecipientID})
	}
	return cond
}

func GetPendingRepositoryTransfers(ctx context.Context, opts *PendingRepositoryTransferOptions) ([]*RepoTransfer, error) {
	transfers := make([]*RepoTransfer, 0, 10)
	return transfers, db.GetEngine(ctx).
		Where(opts.ToConds()).
		Find(&transfers)
}

func IsRepositoryTransferExist(ctx context.Context, repoID int64) (bool, error) {
	return db.GetEngine(ctx).Where("repo_id = ?", repoID).Exist(new(RepoTransfer))
}

// GetPendingRepositoryTransfer fetches the most recent and ongoing transfer
// process for the repository
func GetPendingRepositoryTransfer(ctx context.Context, repo *Repository) (*RepoTransfer, error) {
	transfers, err := GetPendingRepositoryTransfers(ctx, &PendingRepositoryTransferOptions{RepoID: repo.ID})
	if err != nil {
		return nil, err
	}

	if len(transfers) != 1 {
		return nil, ErrNoPendingRepoTransfer{RepoID: repo.ID}
	}

	return transfers[0], nil
}

func DeleteRepositoryTransfer(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(&RepoTransfer{})
	return err
}

// TestRepositoryReadyForTransfer make sure repo is ready to transfer
func TestRepositoryReadyForTransfer(status RepositoryStatus) error {
	switch status {
	case RepositoryBeingMigrated:
		return errors.New("repo is not ready, currently migrating")
	case RepositoryPendingTransfer:
		return ErrRepoTransferInProgress{}
	}
	return nil
}

// CreatePendingRepositoryTransfer transfer a repo from one owner to a new one.
// it marks the repository transfer as "pending"
func CreatePendingRepositoryTransfer(ctx context.Context, doer, newOwner *user_model.User, repoID int64, teams []*organization.Team) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		repo, err := GetRepositoryByID(ctx, repoID)
		if err != nil {
			return err
		}

		if _, err := user_model.GetUserByID(ctx, newOwner.ID); err != nil {
			return err
		}

		// Make sure repo is ready to transfer
		if err := TestRepositoryReadyForTransfer(repo.Status); err != nil {
			return err
		}

		exist, err := IsRepositoryTransferExist(ctx, repo.ID)
		if err != nil {
			return err
		}
		if exist {
			return ErrRepoTransferInProgress{
				Uname: repo.Owner.LowerName,
				Name:  repo.Name,
			}
		}

		repo.Status = RepositoryPendingTransfer
		if err := UpdateRepositoryCols(ctx, repo, "status"); err != nil {
			return err
		}

		// Check if new owner has repository with same name.
		if has, err := IsRepositoryModelExist(ctx, newOwner, repo.Name); err != nil {
			return fmt.Errorf("IsRepositoryExist: %w", err)
		} else if has {
			return ErrRepoAlreadyExist{
				Uname: newOwner.LowerName,
				Name:  repo.Name,
			}
		}

		transfer := &RepoTransfer{
			RepoID:      repo.ID,
			RecipientID: newOwner.ID,
			CreatedUnix: timeutil.TimeStampNow(),
			UpdatedUnix: timeutil.TimeStampNow(),
			DoerID:      doer.ID,
			TeamIDs:     make([]int64, 0, len(teams)),
		}

		for k := range teams {
			transfer.TeamIDs = append(transfer.TeamIDs, teams[k].ID)
		}

		return db.Insert(ctx, transfer)
	})
}
