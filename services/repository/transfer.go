// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
)

func getRepoWorkingLockKey(repoID int64) string {
	return fmt.Sprintf("repo_working_%d", repoID)
}

// AcceptTransferOwnership transfers all corresponding setting from old user to new one.
func AcceptTransferOwnership(ctx context.Context, repo *repo_model.Repository, doer *user_model.User) error {
	releaser, err := globallock.Lock(ctx, getRepoWorkingLockKey(repo.ID))
	if err != nil {
		log.Error("lock.Lock(): %v", err)
		return fmt.Errorf("lock.Lock: %w", err)
	}
	defer releaser()

	repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, repo)
	if err != nil {
		return err
	}

	oldOwnerName := repo.OwnerName

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			return err
		}

		if !repoTransfer.CanUserAcceptOrRejectTransfer(ctx, doer) {
			return util.ErrPermissionDenied
		}

		if err := repo.LoadOwner(ctx); err != nil {
			return err
		}
		for _, team := range repoTransfer.Teams {
			if repoTransfer.Recipient.ID != team.OrgID {
				return fmt.Errorf("team %d does not belong to organization", team.ID)
			}
		}

		return transferOwnership(ctx, repoTransfer.Doer, repoTransfer.Recipient.Name, repo, repoTransfer.Teams)
	}); err != nil {
		return err
	}
	releaser()

	notify_service.TransferRepository(ctx, doer, repo, oldOwnerName)

	return nil
}

// transferOwnership transfers all corresponding repository items from old user to new one.
func transferOwnership(ctx context.Context, doer *user_model.User, newOwnerName string, repo *repo_model.Repository, teams []*organization.Team) (err error) {
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
	if err := repo_model.UpdateRepositoryCols(ctx, repo, "owner_id", "owner_name"); err != nil {
		return fmt.Errorf("update owner: %w", err)
	}

	// Remove redundant collaborators.
	collaborators, _, err := repo_model.GetCollaborators(ctx, &repo_model.FindCollaborationOptions{RepoID: repo.ID})
	if err != nil {
		return fmt.Errorf("GetCollaborators: %w", err)
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

	if oldOwner.IsOrganization() {
		// Remove old team-repository relations.
		if err := organization.RemoveOrgRepo(ctx, oldOwner.ID, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %w", err)
		}

		// Remove project's issues that belong to old organization's projects
		projects, err := project_model.GetAllProjectsIDsByOwnerIDAndType(ctx, oldOwner.ID, project_model.TypeOrganization)
		if err != nil {
			return fmt.Errorf("Unable to find old org projects: %w", err)
		}
		issues, err := issues_model.GetIssueIDsByRepoID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("Unable to find repo's issues: %w", err)
		}
		err = project_model.DeleteAllProjectIssueByIssueIDsAndProjectIDs(ctx, issues, projects)
		if err != nil {
			return fmt.Errorf("Unable to delete project's issues: %w", err)
		}
	}

	if newOwner.IsOrganization() {
		teams, err := organization.FindOrgTeams(ctx, newOwner.ID)
		if err != nil {
			return fmt.Errorf("LoadTeams: %w", err)
		}
		for _, t := range teams {
			if t.IncludesAllRepositories {
				if err := addRepositoryToTeam(ctx, t, repo); err != nil {
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

	if err := repo_model.WatchRepo(ctx, doer, repo, true); err != nil {
		return fmt.Errorf("watchRepo: %w", err)
	}

	if oldOwner.IsOrganization() {
		// Remove watch for organization.
		if err := repo_model.WatchRepo(ctx, oldOwner, repo, false); err != nil {
			return fmt.Errorf("watchRepo [false]: %w", err)
		}

		// Delete labels that belong to the old organization and comments that added these labels
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

	if err := repo_model.DeleteRepositoryTransfer(ctx, repo.ID); err != nil {
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

	newRepo, err := repo_model.GetRepositoryByID(ctx, repo.ID)
	if err != nil {
		return err
	}

	for _, team := range teams {
		if err := addRepositoryToTeam(ctx, team, newRepo); err != nil {
			return err
		}
	}

	return committer.Commit()
}

// changeRepositoryName changes all corresponding setting from old repository name to new one.
func changeRepositoryName(ctx context.Context, repo *repo_model.Repository, newRepoName string) (err error) {
	oldRepoName := repo.Name
	newRepoName = strings.ToLower(newRepoName)
	if err = repo_model.IsUsableRepoName(newRepoName); err != nil {
		return err
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	has, err := repo_model.IsRepositoryModelOrDirExist(ctx, repo.Owner, newRepoName)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %w", err)
	} else if has {
		return repo_model.ErrRepoAlreadyExist{
			Uname: repo.Owner.Name,
			Name:  newRepoName,
		}
	}

	newRepoPath := repo_model.RepoPath(repo.Owner.Name, newRepoName)
	if err = util.Rename(repo.RepoPath(), newRepoPath); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}

	wikiPath := repo.WikiPath()
	isExist, err := util.IsExist(wikiPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", wikiPath, err)
		return err
	}
	if isExist {
		if err = util.Rename(wikiPath, repo_model.WikiPath(repo.Owner.Name, newRepoName)); err != nil {
			return fmt.Errorf("rename repository wiki: %w", err)
		}
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		return repo_model.NewRedirect(ctx, repo.Owner.ID, repo.ID, oldRepoName, newRepoName)
	})
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newRepoName string) error {
	log.Trace("ChangeRepositoryName: %s/%s -> %s", doer.Name, repo.Name, newRepoName)

	oldRepoName := repo.Name

	// Change repository directory name. We must lock the local copy of the
	// repo so that we can automatically rename the repo path and updates the
	// local copy's origin accordingly.

	releaser, err := globallock.Lock(ctx, getRepoWorkingLockKey(repo.ID))
	if err != nil {
		log.Error("lock.Lock(): %v", err)
		return fmt.Errorf("lock.Lock: %w", err)
	}
	defer releaser()

	if err := changeRepositoryName(ctx, repo, newRepoName); err != nil {
		return err
	}
	releaser()

	repo.Name = newRepoName
	notify_service.RenameRepository(ctx, doer, repo, oldRepoName)

	return nil
}

// StartRepositoryTransfer transfer a repo from one owner to a new one.
// it make repository into pending transfer state, if doer can not create repo for new owner.
func StartRepositoryTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository, teams []*organization.Team) error {
	releaser, err := globallock.Lock(ctx, getRepoWorkingLockKey(repo.ID))
	if err != nil {
		return fmt.Errorf("lock.Lock: %w", err)
	}
	defer releaser()

	if err := repo_model.TestRepositoryReadyForTransfer(repo.Status); err != nil {
		return err
	}

	var isDirectTransfer bool
	oldOwnerName := repo.OwnerName

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// Admin is always allowed to transfer || user transfer repo back to his account,
		// then it will transfer directly without acceptance.
		if doer.IsAdmin || doer.ID == newOwner.ID {
			isDirectTransfer = true
			return transferOwnership(ctx, doer, newOwner.Name, repo, teams)
		}

		if user_model.IsUserBlockedBy(ctx, doer, newOwner.ID) {
			return user_model.ErrBlockedUser
		}

		// If new owner is an org and user can create repos he can transfer directly too
		if newOwner.IsOrganization() {
			allowed, err := organization.CanCreateOrgRepo(ctx, newOwner.ID, doer.ID)
			if err != nil {
				return err
			}
			if allowed {
				isDirectTransfer = true
				return transferOwnership(ctx, doer, newOwner.Name, repo, teams)
			}
		}

		// In case the new owner would not have sufficient access to the repo, give access rights for read
		hasAccess, err := access_model.HasAnyUnitAccess(ctx, newOwner.ID, repo)
		if err != nil {
			return err
		}
		if !hasAccess {
			if err := AddOrUpdateCollaborator(ctx, repo, newOwner, perm.AccessModeRead); err != nil {
				return err
			}
		}

		// Make repo as pending for transfer
		repo.Status = repo_model.RepositoryPendingTransfer
		return repo_model.CreatePendingRepositoryTransfer(ctx, doer, newOwner, repo.ID, teams)
	}); err != nil {
		return err
	}

	if isDirectTransfer {
		notify_service.TransferRepository(ctx, doer, repo, oldOwnerName)
	} else {
		// notify users who are able to accept / reject transfer
		notify_service.RepoPendingTransfer(ctx, doer, newOwner, repo)
	}

	return nil
}

// RejectRepositoryTransfer marks the repository as ready and remove pending transfer entry,
// thus cancel the transfer process.
// The accepter can reject the transfer.
func RejectRepositoryTransfer(ctx context.Context, repo *repo_model.Repository, doer *user_model.User) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, repo)
		if err != nil {
			return err
		}

		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			return err
		}

		if !repoTransfer.CanUserAcceptOrRejectTransfer(ctx, doer) {
			return util.ErrPermissionDenied
		}

		repo.Status = repo_model.RepositoryReady
		if err := repo_model.UpdateRepositoryCols(ctx, repo, "status"); err != nil {
			return err
		}

		return repo_model.DeleteRepositoryTransfer(ctx, repo.ID)
	})
}

func canUserCancelTransfer(ctx context.Context, r *repo_model.RepoTransfer, u *user_model.User) bool {
	if u.IsAdmin || u.ID == r.DoerID {
		return true
	}

	if err := r.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return false
	}

	if err := r.Repo.LoadOwner(ctx); err != nil {
		log.Error("LoadOwner: %v", err)
		return false
	}

	if !r.Repo.Owner.IsOrganization() {
		return r.Repo.OwnerID == u.ID
	}

	perm, err := access_model.GetUserRepoPermission(ctx, r.Repo, u)
	if err != nil {
		log.Error("GetUserRepoPermission: %v", err)
		return false
	}
	return perm.IsOwner()
}

// CancelRepositoryTransfer cancels the repository transfer process. The sender or
// the users who have admin permission of the original repository can cancel the transfer
func CancelRepositoryTransfer(ctx context.Context, repoTransfer *repo_model.RepoTransfer, doer *user_model.User) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			return err
		}

		if !canUserCancelTransfer(ctx, repoTransfer, doer) {
			return util.ErrPermissionDenied
		}

		repoTransfer.Repo.Status = repo_model.RepositoryReady
		if err := repo_model.UpdateRepositoryCols(ctx, repoTransfer.Repo, "status"); err != nil {
			return err
		}

		return repo_model.DeleteRepositoryTransfer(ctx, repoTransfer.RepoID)
	})
}
