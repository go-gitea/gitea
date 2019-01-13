package models

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/util"
	"github.com/Unknwon/com"
)

type RepoStatus uint8

const (
	Rejected RepoStatus = iota
	Pending
	Accepted
)

// RepoTransfer is used to manage repository transfers
type RepoTransfer struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64
	RecipientID int64
	Recipient   *User `xorm:"-"`
	RepoID      int64
	CreatedUnix util.TimeStamp `xorm:"INDEX NOT NULL created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX NOT NULL updated"`
	Status      RepoStatus
}

// LoadRecipient fetches the transfer recipient from the database
func (r *RepoTransfer) LoadRecipient() error {
	if r.Recipient != nil {
		return nil
	}

	u, err := GetUserByID(r.RecipientID)
	if err != nil {
		return err
	}

	r.Recipient = u
	return nil
}

// GetPendingRepositoryTransfer fetches the most recent and ongoing transfer
// process for the repository
func GetPendingRepositoryTransfer(repo *Repository, doer *User) (*RepoTransfer, error) {
	var transfer = new(RepoTransfer)

	_, err := x.Where("status = ? AND repo_id = ? AND user_id = ?", Pending, repo.ID, doer.ID).
		Get(transfer)

	if err != nil {
		return nil, err
	}

	if transfer.ID == 0 {
		return nil, ErrNoPendingRepoTransfer{RepoID: repo.ID, UserID: doer.ID}
	}

	return transfer, nil
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
	// It has to be cancelled for a new transfer to occur

	n, err := x.Count(&RepoTransfer{
		RepoID: repo.ID,
		UserID: doer.ID,
		Status: Pending,
	})
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
		UserID:      doer.ID,
		RecipientID: newOwner.ID,
		Status:      Pending,
		CreatedUnix: util.TimeStampNow(),
		UpdatedUnix: util.TimeStampNow(),
	}

	_, err = x.Insert(transfer)
	return err
}

// TransferOwnership transfers all corresponding setting from old user to new one.
func TransferOwnership(doer *User, newOwnerName string, repo *Repository) error {
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
