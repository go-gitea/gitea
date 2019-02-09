// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
)

// TransferStatus determines the current state of a transfer
type TransferStatus uint8

const (
	// Pending is the default repo transfer state. All initiated transfers
	// automatically get this status.
	Pending TransferStatus = iota
	// Rejected is a status for transfers that get cancelled by either the
	// recipient or the user who initiated the transfer
	Rejected
	// Accepted is a repo transfer state for repository transfers that have
	// been acknowledged by the recipient
	Accepted
)

// RepoTransfer is used to manage repository transfers
type RepoTransfer struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64
	User        *User `xorm:"-"`
	RecipientID int64
	Recipient   *User `xorm:"-"`
	RepoID      int64
	CreatedUnix util.TimeStamp `xorm:"INDEX NOT NULL created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX NOT NULL updated"`
	Status      TransferStatus
}

// LoadAttributes fetches the transfer recipient from the database
func (r *RepoTransfer) LoadAttributes() error {
	if r.Recipient != nil && r.User != nil {
		return nil
	}

	if r.Recipient == nil {
		u, err := GetUserByID(r.RecipientID)
		if err != nil {
			return err
		}

		r.Recipient = u
	}

	if r.User == nil {
		u, err := GetUserByID(r.UserID)
		if err != nil {
			return err
		}

		r.User = u
	}

	return nil
}

// GetPendingRepositoryTransfer fetches the most recent and ongoing transfer
// process for the repository
func GetPendingRepositoryTransfer(repo *Repository) (*RepoTransfer, error) {
	var transfer = new(RepoTransfer)

	has, err := x.Where("status = ? AND repo_id = ? ", Pending, repo.ID).
		Get(transfer)
	if err != nil {
		return nil, err
	}

	if transfer.ID == 0 || !has {
		return nil, ErrNoPendingRepoTransfer{RepoID: repo.ID}
	}

	return transfer, nil
}

func acceptRepositoryTransfer(repo *Repository) error {
	_, err := x.Where("repo_id = ?", repo.ID).Cols("status").Update(&RepoTransfer{
		Status: Accepted,
	})
	return err
}

// CancelRepositoryTransfer makes sure to set the transfer process as
// "rejected". Thus ending the transfer process
func CancelRepositoryTransfer(repoTransfer *RepoTransfer) error {
	repoTransfer.Status = Rejected
	repoTransfer.UpdatedUnix = util.TimeStampNow()
	_, err := x.ID(repoTransfer.ID).Cols("updated_unix", "status").
		Update(repoTransfer)
	return err
}

// StartRepositoryTransfer marks the repository transfer as "pending". It
// doesn't actually transfer the repository until the user acks the transfer.
func StartRepositoryTransfer(doer *User, newOwnerName string, repo *Repository) error {
	// Make sure the repo isn't being transferred to someone currently
	// Only one transfer process can be initiated at a time.
	// It has to be cancelled for a new one to occur
	n, err := x.Where("status = ? AND repo_id = ?", Pending, repo.ID).
		Count(new(RepoTransfer))
	if err != nil {
		return err
	}

	if n > 0 {
		return ErrRepoTransferInProgress{newOwnerName, repo.Name}
	}

	newOwner, err := GetUserByName(newOwnerName)
	if err != nil {
		return fmt.Errorf("get new owner '%s': %v", newOwnerName, err)
	}

	// Check if new owner has repository with same name.
	has, err := IsRepositoryExist(newOwner, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{newOwnerName, repo.Name}
	}

	transfer := &RepoTransfer{
		RepoID:      repo.ID,
		RecipientID: newOwner.ID,
		Status:      Pending,
		CreatedUnix: util.TimeStampNow(),
		UpdatedUnix: util.TimeStampNow(),
		UserID:      doer.ID,
	}

	_, err = x.Insert(transfer)
	return err
}

// TransferOwnership transfers all corresponding setting from one user to
// another.
func TransferOwnership(doer, newOwner *User, repo *Repository) error {

	// Check if new owner has repository with same name.
	has, err := IsRepositoryExist(newOwner, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{newOwner.Name, repo.Name}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return fmt.Errorf("sess.Begin: %v", err)
	}

	owner := repo.Owner

	// Note: we have to set value here to make sure recalculate accesses is based on
	// new owner.
	repo.OwnerID = newOwner.ID
	repo.Owner = newOwner

	// Update repository.
	if _, err := sess.ID(repo.ID).Update(repo); err != nil {
		return fmt.Errorf("update owner: %v", err)
	}

	if err := acceptRepositoryTransfer(repo); err != nil {
		return err
	}

	// Remove redundant collaborators.
	collaborators, err := repo.getCollaborators(sess)
	if err != nil {
		return fmt.Errorf("getCollaborators: %v", err)
	}

	// Dummy object.
	collaboration := &Collaboration{RepoID: repo.ID}
	for _, c := range collaborators {
		if c.ID != newOwner.ID {
			isMember, err := newOwner.IsOrgMember(c.ID)
			if err != nil {
				return fmt.Errorf("IsOrgMember: %v", err)
			} else if !isMember {
				continue
			}
		}
		collaboration.UserID = c.ID
		if _, err = sess.Delete(collaboration); err != nil {
			return fmt.Errorf("remove collaborator '%d': %v", c.ID, err)
		}
	}

	// Remove old team-repository relations.
	if owner.IsOrganization() {
		if err = owner.removeOrgRepo(sess, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %v", err)
		}
	}

	if newOwner.IsOrganization() {
		t, err := newOwner.getOwnerTeam(sess)
		if err != nil {
			return fmt.Errorf("getOwnerTeam: %v", err)
		} else if err = t.addRepository(sess, repo); err != nil {
			return fmt.Errorf("add to owner team: %v", err)
		}
	} else {
		// Organization called this in addRepository method.
		if err = repo.recalculateAccesses(sess); err != nil {
			return fmt.Errorf("recalculateAccesses: %v", err)
		}
	}

	// Update repository count.
	if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos+1 WHERE id=?", newOwner.ID); err != nil {
		return fmt.Errorf("increase new owner repository count: %v", err)
	} else if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", owner.ID); err != nil {
		return fmt.Errorf("decrease old owner repository count: %v", err)
	}

	if err = watchRepo(sess, doer.ID, repo.ID, true); err != nil {
		return fmt.Errorf("watchRepo: %v", err)
	} else if err = transferRepoAction(sess, doer, owner, repo); err != nil {
		return fmt.Errorf("transferRepoAction: %v", err)
	}

	// Rename remote repository to new path and delete local copy.
	dir := UserPath(newOwner.Name)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = os.Rename(RepoPath(owner.Name, repo.Name), RepoPath(newOwner.Name, repo.Name)); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}
	RemoveAllWithNotice("Delete repository local copy", repo.LocalCopyPath())

	// Rename remote wiki repository to new path and delete local copy.
	wikiPath := WikiPath(owner.Name, repo.Name)
	if com.IsExist(wikiPath) {
		RemoveAllWithNotice("Delete repository wiki local copy", repo.LocalWikiPath())
		if err = os.Rename(wikiPath, WikiPath(newOwner.Name, repo.Name)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	return sess.Commit()
}
