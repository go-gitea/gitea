// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// RepoTransfer is used to manage repository transfers
type RepoTransfer struct {
	ID          int64 `xorm:"pk autoincr"`
	DoerID      int64
	Doer        *User `xorm:"-"`
	RecipientID int64
	Recipient   *User `xorm:"-"`
	RepoID      int64
	TeamIDs     []int64
	Teams       []*Team `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL updated"`
}

// LoadAttributes fetches the transfer recipient from the database
func (r *RepoTransfer) LoadAttributes() error {
	if r.Recipient == nil {
		u, err := GetUserByID(r.RecipientID)
		if err != nil {
			return err
		}

		r.Recipient = u
	}

	if r.Recipient.IsOrganization() && len(r.TeamIDs) != len(r.Teams) {

		for _, v := range r.TeamIDs {
			team, err := GetTeamByID(v)
			if err != nil {
				return err
			}

			if team.OrgID != r.Recipient.ID {
				return fmt.Errorf("team %d belongs not to org %d", v, r.Recipient.ID)
			}

			r.Teams = append(r.Teams, team)
		}
	}

	if r.Doer == nil {
		u, err := GetUserByID(r.DoerID)
		if err != nil {
			return err
		}

		r.Doer = u
	}

	return nil
}

// IsTransferForUser checks if the user has the rights to accept/decline a repo
// transfer.
// For organizations, it checks if the user is able to create repos
func (r *RepoTransfer) IsTransferForUser(u *User) bool {
	if err := r.LoadAttributes(); err != nil {
		return false
	}

	if !r.Recipient.IsOrganization() {
		return r.RecipientID == u.ID
	}

	allowed, err := CanCreateOrgRepo(r.RecipientID, u.ID)
	if err != nil {
		log.Error("CanCreateOrgRepo: %v", err)
		return false
	}

	return allowed
}

// GetPendingRepositoryTransfer fetches the most recent and ongoing transfer
// process for the repository
func GetPendingRepositoryTransfer(repo *Repository) (*RepoTransfer, error) {
	var transfer = new(RepoTransfer)

	has, err := x.Where("repo_id = ? ", repo.ID).Get(transfer)
	if err != nil {
		return nil, err
	}

	if transfer.ID == 0 || !has {
		return nil, ErrNoPendingRepoTransfer{RepoID: repo.ID}
	}

	return transfer, nil
}

func deleteRepositoryTransfer(e Engine, repoID int64) error {
	_, err := e.Where("repo_id = ?", repoID).Delete(&RepoTransfer{})
	return err
}

// CancelRepositoryTransfer makes sure to set the transfer process as
// "rejected". Thus ending the transfer process
func CancelRepositoryTransfer(repo *Repository) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()

	repo.Status = RepositoryReady
	if err := updateRepositoryCols(sess, repo, "status"); err != nil {
		return err
	}

	if err := deleteRepositoryTransfer(sess, repo.ID); err != nil {
		return err
	}

	return sess.Commit()
}

// TestRepositoryReadyForTransfer make sure repo is ready to transfer
func TestRepositoryReadyForTransfer(status RepositoryStatus) error {
	switch status {
	case RepositoryBeingMigrated:
		return fmt.Errorf("repo is not ready, currently migrating")
	case RepositoryPendingTransfer:
		return ErrRepoTransferInProgress{}
	}
	return nil
}

// StartRepositoryTransfer transfer a repo from one owner to a new one.
// it marks the repository transfer as "pending",
// if the new owner is a user or if he dont have access to the new place.
func StartRepositoryTransfer(doer, newOwner *User, repoID int64, teams []*Team) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()
	if err := startRepositoryTransfer(sess, doer, newOwner, repoID, teams); err != nil {
		return err
	}
	return sess.Commit()
}

func startRepositoryTransfer(e Engine, doer, newOwner *User, repoID int64, teams []*Team) error {
	repo, err := getRepositoryByID(e, repoID)
	if err != nil {
		return err
	}

	// Make sure repo is ready to transfer
	if err = TestRepositoryReadyForTransfer(repo.Status); err != nil {
		return err
	}

	repo.Status = RepositoryPendingTransfer
	if err = updateRepositoryCols(e, repo, "status"); err != nil {
		return err
	}

	// Check if new owner has repository with same name.
	has, err := IsRepositoryExist(newOwner, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{newOwner.LowerName, repo.Name}
	}

	transfer := &RepoTransfer{
		RepoID:      repo.ID,
		RecipientID: newOwner.ID,
		CreatedUnix: timeutil.TimeStampNow(),
		UpdatedUnix: timeutil.TimeStampNow(),
		DoerID:      doer.ID,
		TeamIDs:     []int64{},
	}

	for k := range teams {
		transfer.TeamIDs = append(transfer.TeamIDs, teams[k].ID)
	}

	_, err = e.Insert(transfer)
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

	oldOwner := repo.Owner

	// Note: we have to set value here to make sure recalculate accesses is based on
	// new owner.
	repo.OwnerID = newOwner.ID
	repo.Owner = newOwner
	repo.OwnerName = newOwner.Name

	// Update repository.
	if _, err := sess.ID(repo.ID).Update(repo); err != nil {
		return fmt.Errorf("update owner: %v", err)
	}

	// Remove redundant collaborators.
	collaborators, err := repo.getCollaborators(sess, ListOptions{})
	if err != nil {
		return fmt.Errorf("getCollaborators: %v", err)
	}

	// Dummy object.
	collaboration := &Collaboration{RepoID: repo.ID}
	for _, c := range collaborators {
		if c.ID != newOwner.ID {
			isMember, err := isOrganizationMember(sess, newOwner.ID, c.ID)
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
	if oldOwner.IsOrganization() {
		if err = oldOwner.removeOrgRepo(sess, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %v", err)
		}
	}

	if newOwner.IsOrganization() {
		if err := newOwner.getTeams(sess); err != nil {
			return fmt.Errorf("GetTeams: %v", err)
		}
		for _, t := range newOwner.Teams {
			if t.IncludesAllRepositories {
				if err := t.addRepository(sess, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}
	} else if err = repo.recalculateAccesses(sess); err != nil {
		// Organization called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	// Update repository count.
	if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos+1 WHERE id=?", newOwner.ID); err != nil {
		return fmt.Errorf("increase new owner repository count: %v", err)
	} else if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", oldOwner.ID); err != nil {
		return fmt.Errorf("decrease old owner repository count: %v", err)
	}

	if err = watchRepo(sess, doer.ID, repo.ID, true); err != nil {
		return fmt.Errorf("watchRepo: %v", err)
	}

	// Remove watch for organization.
	if oldOwner.IsOrganization() {
		if err = watchRepo(sess, oldOwner.ID, repo.ID, false); err != nil {
			return fmt.Errorf("watchRepo [false]: %v", err)
		}
	}

	// Rename remote repository to new path and delete local copy.
	dir := UserPath(newOwner.Name)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = os.Rename(RepoPath(oldOwner.Name, repo.Name), RepoPath(newOwner.Name, repo.Name)); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}

	// Rename remote wiki repository to new path and delete local copy.
	wikiPath := WikiPath(oldOwner.Name, repo.Name)
	isExist, err := util.IsExist(wikiPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", wikiPath, err)
		return err
	}
	if isExist {
		if err = os.Rename(wikiPath, WikiPath(newOwner.Name, repo.Name)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	if err = deleteRepositoryTransfer(sess, repo.ID); err != nil {
		return fmt.Errorf("accept repository transfer: %v", err)
	}

	// If there was previously a redirect at this location, remove it.
	if err = deleteRepoRedirect(sess, newOwner.ID, repo.Name); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	if err := newRepoRedirect(sess, oldOwner.ID, repo.ID, repo.Name, repo.Name); err != nil {
		return fmt.Errorf("newRepoRedirect: %v", err)
	}

	return sess.Commit()
}
