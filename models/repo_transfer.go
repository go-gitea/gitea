// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"
	"os"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// RepoTransfer is used to manage repository transfers
type RepoTransfer struct {
	ID          int64 `xorm:"pk autoincr"`
	DoerID      int64
	Doer        *user_model.User `xorm:"-"`
	RecipientID int64
	Recipient   *user_model.User `xorm:"-"`
	RepoID      int64
	TeamIDs     []int64
	Teams       []*organization.Team `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL updated"`
}

func init() {
	db.RegisterModel(new(RepoTransfer))
}

// LoadAttributes fetches the transfer recipient from the database
func (r *RepoTransfer) LoadAttributes(ctx context.Context) error {
	if r.Recipient == nil {
		u, err := user_model.GetUserByID(ctx, r.RecipientID)
		if err != nil {
			return err
		}

		r.Recipient = u
	}

	if r.Recipient.IsOrganization() && len(r.TeamIDs) != len(r.Teams) {
		for _, v := range r.TeamIDs {
			team, err := organization.GetTeamByID(ctx, v)
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
		u, err := user_model.GetUserByID(ctx, r.DoerID)
		if err != nil {
			return err
		}

		r.Doer = u
	}

	return nil
}

// CanUserAcceptTransfer checks if the user has the rights to accept/decline a repo transfer.
// For user, it checks if it's himself
// For organizations, it checks if the user is able to create repos
func (r *RepoTransfer) CanUserAcceptTransfer(ctx context.Context, u *user_model.User) bool {
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

// GetPendingRepositoryTransfer fetches the most recent and ongoing transfer
// process for the repository
func GetPendingRepositoryTransfer(ctx context.Context, repo *repo_model.Repository) (*RepoTransfer, error) {
	transfer := new(RepoTransfer)

	has, err := db.GetEngine(ctx).Where("repo_id = ? ", repo.ID).Get(transfer)
	if err != nil {
		return nil, err
	}

	if !has {
		return nil, ErrNoPendingRepoTransfer{RepoID: repo.ID}
	}

	return transfer, nil
}

func deleteRepositoryTransfer(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(&RepoTransfer{})
	return err
}

// CancelRepositoryTransfer marks the repository as ready and remove pending transfer entry,
// thus cancel the transfer process.
func CancelRepositoryTransfer(ctx context.Context, repo *repo_model.Repository) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	repo.Status = repo_model.RepositoryReady
	if err := repo_model.UpdateRepositoryCols(ctx, repo, "status"); err != nil {
		return err
	}

	if err := deleteRepositoryTransfer(ctx, repo.ID); err != nil {
		return err
	}

	return committer.Commit()
}

// TestRepositoryReadyForTransfer make sure repo is ready to transfer
func TestRepositoryReadyForTransfer(status repo_model.RepositoryStatus) error {
	switch status {
	case repo_model.RepositoryBeingMigrated:
		return fmt.Errorf("repo is not ready, currently migrating")
	case repo_model.RepositoryPendingTransfer:
		return ErrRepoTransferInProgress{}
	}
	return nil
}

// CreatePendingRepositoryTransfer transfer a repo from one owner to a new one.
// it marks the repository transfer as "pending"
func CreatePendingRepositoryTransfer(ctx context.Context, doer, newOwner *user_model.User, repoID int64, teams []*organization.Team) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		repo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil {
			return err
		}

		// Make sure repo is ready to transfer
		if err := TestRepositoryReadyForTransfer(repo.Status); err != nil {
			return err
		}

		repo.Status = repo_model.RepositoryPendingTransfer
		if err := repo_model.UpdateRepositoryCols(ctx, repo, "status"); err != nil {
			return err
		}

		// Check if new owner has repository with same name.
		if has, err := repo_model.IsRepositoryModelExist(ctx, newOwner, repo.Name); err != nil {
			return fmt.Errorf("IsRepositoryExist: %w", err)
		} else if has {
			return repo_model.ErrRepoAlreadyExist{
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

// TransferOwnership transfers all corresponding repository items from old user to new one.
func TransferOwnership(ctx context.Context, doer *user_model.User, newOwnerName string, repo *repo_model.Repository) (err error) {
	repoRenamed := false
	wikiRenamed := false
	oldOwnerName := doer.Name

	defer func() {
		if !repoRenamed && !wikiRenamed {
			return
		}

		recoverErr := recover()
		if err == nil && recoverErr == nil {
			return
		}

		if repoRenamed {
			if err := util.Rename(repo_model.RepoPath(newOwnerName, repo.Name), repo_model.RepoPath(oldOwnerName, repo.Name)); err != nil {
				log.Critical("Unable to move repository %s/%s directory from %s back to correct place %s: %v", oldOwnerName, repo.Name,
					repo_model.RepoPath(newOwnerName, repo.Name), repo_model.RepoPath(oldOwnerName, repo.Name), err)
			}
		}

		if wikiRenamed {
			if err := util.Rename(repo_model.WikiPath(newOwnerName, repo.Name), repo_model.WikiPath(oldOwnerName, repo.Name)); err != nil {
				log.Critical("Unable to move wiki for repository %s/%s directory from %s back to correct place %s: %v", oldOwnerName, repo.Name,
					repo_model.WikiPath(newOwnerName, repo.Name), repo_model.WikiPath(oldOwnerName, repo.Name), err)
			}
		}

		if recoverErr != nil {
			log.Error("Panic within TransferOwnership: %v\n%s", recoverErr, log.Stack(2))
			panic(recoverErr)
		}
	}()

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	newOwner, err := user_model.GetUserByName(ctx, newOwnerName)
	if err != nil {
		return fmt.Errorf("get new owner '%s': %w", newOwnerName, err)
	}
	newOwnerName = newOwner.Name // ensure capitalisation matches

	// Check if new owner has repository with same name.
	if has, err := repo_model.IsRepositoryModelOrDirExist(ctx, newOwner, repo.Name); err != nil {
		return fmt.Errorf("IsRepositoryExist: %w", err)
	} else if has {
		return repo_model.ErrRepoAlreadyExist{
			Uname: newOwnerName,
			Name:  repo.Name,
		}
	}

	oldOwner := repo.Owner
	oldOwnerName = oldOwner.Name

	// Note: we have to set value here to make sure recalculate accesses is based on
	// new owner.
	repo.OwnerID = newOwner.ID
	repo.Owner = newOwner
	repo.OwnerName = newOwner.Name

	// Update repository.
	if _, err := sess.ID(repo.ID).Update(repo); err != nil {
		return fmt.Errorf("update owner: %w", err)
	}

	// Remove redundant collaborators.
	collaborators, err := repo_model.GetCollaborators(ctx, repo.ID, db.ListOptions{})
	if err != nil {
		return fmt.Errorf("getCollaborators: %w", err)
	}

	// Dummy object.
	collaboration := &repo_model.Collaboration{RepoID: repo.ID}
	for _, c := range collaborators {
		if c.IsGhost() {
			collaboration.ID = c.Collaboration.ID
			if _, err := sess.Delete(collaboration); err != nil {
				return fmt.Errorf("remove collaborator '%d': %w", c.ID, err)
			}
			collaboration.ID = 0
		}

		if c.ID != newOwner.ID {
			isMember, err := organization.IsOrganizationMember(ctx, newOwner.ID, c.ID)
			if err != nil {
				return fmt.Errorf("IsOrgMember: %w", err)
			} else if !isMember {
				continue
			}
		}
		collaboration.UserID = c.ID
		if _, err := sess.Delete(collaboration); err != nil {
			return fmt.Errorf("remove collaborator '%d': %w", c.ID, err)
		}
		collaboration.UserID = 0
	}

	// Remove old team-repository relations.
	if oldOwner.IsOrganization() {
		if err := organization.RemoveOrgRepo(ctx, oldOwner.ID, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %w", err)
		}
	}

	if newOwner.IsOrganization() {
		teams, err := organization.FindOrgTeams(ctx, newOwner.ID)
		if err != nil {
			return fmt.Errorf("LoadTeams: %w", err)
		}
		for _, t := range teams {
			if t.IncludesAllRepositories {
				if err := AddRepository(ctx, t, repo); err != nil {
					return fmt.Errorf("AddRepository: %w", err)
				}
			}
		}
	} else if err := access_model.RecalculateAccesses(ctx, repo); err != nil {
		// Organization called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %w", err)
	}

	// Update repository count.
	if _, err := sess.Exec("UPDATE `user` SET num_repos=num_repos+1 WHERE id=?", newOwner.ID); err != nil {
		return fmt.Errorf("increase new owner repository count: %w", err)
	} else if _, err := sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", oldOwner.ID); err != nil {
		return fmt.Errorf("decrease old owner repository count: %w", err)
	}

	if err := repo_model.WatchRepo(ctx, doer.ID, repo.ID, true); err != nil {
		return fmt.Errorf("watchRepo: %w", err)
	}

	// Remove watch for organization.
	if oldOwner.IsOrganization() {
		if err := repo_model.WatchRepo(ctx, oldOwner.ID, repo.ID, false); err != nil {
			return fmt.Errorf("watchRepo [false]: %w", err)
		}
	}

	// Delete labels that belong to the old organization and comments that added these labels
	if oldOwner.IsOrganization() {
		if _, err := sess.Exec(`DELETE FROM issue_label WHERE issue_label.id IN (
			SELECT il_too.id FROM (
				SELECT il_too_too.id
					FROM issue_label AS il_too_too
						INNER JOIN label ON il_too_too.label_id = label.id
						INNER JOIN issue on issue.id = il_too_too.issue_id
					WHERE
						issue.repo_id = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != ?))
		) AS il_too )`, repo.ID, newOwner.ID); err != nil {
			return fmt.Errorf("Unable to remove old org labels: %w", err)
		}

		if _, err := sess.Exec(`DELETE FROM comment WHERE comment.id IN (
			SELECT il_too.id FROM (
				SELECT com.id
					FROM comment AS com
						INNER JOIN label ON com.label_id = label.id
						INNER JOIN issue ON issue.id = com.issue_id
					WHERE
						com.type = ? AND issue.repo_id = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != ?))
		) AS il_too)`, issues_model.CommentTypeLabel, repo.ID, newOwner.ID); err != nil {
			return fmt.Errorf("Unable to remove old org label comments: %w", err)
		}
	}

	// Rename remote repository to new path and delete local copy.
	dir := user_model.UserPath(newOwner.Name)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %w", dir, err)
	}

	if err := util.Rename(repo_model.RepoPath(oldOwner.Name, repo.Name), repo_model.RepoPath(newOwner.Name, repo.Name)); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	repoRenamed = true

	// Rename remote wiki repository to new path and delete local copy.
	wikiPath := repo_model.WikiPath(oldOwner.Name, repo.Name)

	if isExist, err := util.IsExist(wikiPath); err != nil {
		log.Error("Unable to check if %s exists. Error: %v", wikiPath, err)
		return err
	} else if isExist {
		if err := util.Rename(wikiPath, repo_model.WikiPath(newOwner.Name, repo.Name)); err != nil {
			return fmt.Errorf("rename repository wiki: %w", err)
		}
		wikiRenamed = true
	}

	if err := deleteRepositoryTransfer(ctx, repo.ID); err != nil {
		return fmt.Errorf("deleteRepositoryTransfer: %w", err)
	}
	repo.Status = repo_model.RepositoryReady
	if err := repo_model.UpdateRepositoryCols(ctx, repo, "status"); err != nil {
		return err
	}

	// If there was previously a redirect at this location, remove it.
	if err := repo_model.DeleteRedirect(ctx, newOwner.ID, repo.Name); err != nil {
		return fmt.Errorf("delete repo redirect: %w", err)
	}

	if err := repo_model.NewRedirect(ctx, oldOwner.ID, repo.ID, repo.Name, repo.Name); err != nil {
		return fmt.Errorf("repo_model.NewRedirect: %w", err)
	}

	return committer.Commit()
}
