// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	activities_model "code.gitea.io/gitea/models/activities"
	admin_model "code.gitea.io/gitea/models/admin"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/storage"
	pull_service "code.gitea.io/gitea/services/pull"

	"xorm.io/builder"
)

// DeleteRepository deletes a repository for a user or organization.
func DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, notify bool) error {
	if err := pull_service.CloseRepoBranchesPulls(ctx, doer, repo); err != nil {
		log.Error("CloseRepoBranchesPulls failed: %v", err)
	}

	if notify {
		// If the repo itself has webhooks, we need to trigger them before deleting it...
		notification.NotifyDeleteRepository(ctx, doer, repo)
	}

	if err := DeleteRepositoryWithNoNotify(doer, repo.OwnerID, repo.ID); err != nil {
		return err
	}

	return packages_model.UnlinkRepositoryFromAllPackages(ctx, repo.ID)
}

// DeleteRepositoryWithNoNotify deletes a repository for a user or organization.
// make sure if you call this func to close open sessions (sqlite will otherwise get a deadlock)
func DeleteRepositoryWithNoNotify(doer *user_model.User, uid, repoID int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	// In case is a organization.
	org, err := user_model.GetUserByID(ctx, uid)
	if err != nil {
		return err
	}

	repo := &repo_model.Repository{OwnerID: uid}
	has, err := sess.ID(repoID).Get(repo)
	if err != nil {
		return err
	} else if !has {
		return repo_model.ErrRepoNotExist{
			ID:        repoID,
			UID:       uid,
			OwnerName: "",
			Name:      "",
		}
	}

	// Delete Deploy Keys
	deployKeys, err := asymkey_model.ListDeployKeys(ctx, &asymkey_model.ListDeployKeysOptions{RepoID: repoID})
	if err != nil {
		return fmt.Errorf("listDeployKeys: %w", err)
	}
	needRewriteKeysFile := len(deployKeys) > 0
	for _, dKey := range deployKeys {
		if err := models.DeleteDeployKey(ctx, doer, dKey.ID); err != nil {
			return fmt.Errorf("deleteDeployKeys: %w", err)
		}
	}

	if cnt, err := sess.ID(repoID).Delete(&repo_model.Repository{}); err != nil {
		return err
	} else if cnt != 1 {
		return repo_model.ErrRepoNotExist{
			ID:        repoID,
			UID:       uid,
			OwnerName: "",
			Name:      "",
		}
	}

	if org.IsOrganization() {
		teams, err := organization.FindOrgTeams(ctx, org.ID)
		if err != nil {
			return err
		}
		for _, t := range teams {
			if !organization.HasTeamRepo(ctx, t.OrgID, t.ID, repoID) {
				continue
			} else if err = models.RemoveRepositoryFromTeam(ctx, t, repo, false); err != nil {
				return err
			}
		}
	}

	attachments := make([]*repo_model.Attachment, 0, 20)
	if err = sess.Join("INNER", "`release`", "`release`.id = `attachment`.release_id").
		Where("`release`.repo_id = ?", repoID).
		Find(&attachments); err != nil {
		return err
	}
	releaseAttachments := make([]string, 0, len(attachments))
	for i := 0; i < len(attachments); i++ {
		releaseAttachments = append(releaseAttachments, attachments[i].RelativePath())
	}

	if _, err := db.Exec(ctx, "UPDATE `user` SET num_stars=num_stars-1 WHERE id IN (SELECT `uid` FROM `star` WHERE repo_id = ?)", repo.ID); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).In("hook_id", builder.Select("id").From("webhook").Where(builder.Eq{"webhook.repo_id": repo.ID})).
		Delete(&webhook.HookTask{}); err != nil {
		return err
	}

	if err := db.DeleteBeans(ctx,
		&access_model.Access{RepoID: repo.ID},
		&activities_model.Action{RepoID: repo.ID},
		&repo_model.Collaboration{RepoID: repoID},
		&issues_model.Comment{RefRepoID: repoID},
		&git_model.CommitStatus{RepoID: repoID},
		&git_model.DeletedBranch{RepoID: repoID},
		&git_model.LFSLock{RepoID: repoID},
		&repo_model.LanguageStat{RepoID: repoID},
		&issues_model.Milestone{RepoID: repoID},
		&repo_model.Mirror{RepoID: repoID},
		&activities_model.Notification{RepoID: repoID},
		&git_model.ProtectedBranch{RepoID: repoID},
		&git_model.ProtectedTag{RepoID: repoID},
		&repo_model.PushMirror{RepoID: repoID},
		&repo_model.Release{RepoID: repoID},
		&repo_model.RepoIndexerStatus{RepoID: repoID},
		&repo_model.Redirect{RedirectRepoID: repoID},
		&repo_model.RepoUnit{RepoID: repoID},
		&repo_model.Star{RepoID: repoID},
		&admin_model.Task{RepoID: repoID},
		&repo_model.Watch{RepoID: repoID},
		&webhook.Webhook{RepoID: repoID},
		&secret_model.Secret{RepoID: repoID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %w", err)
	}

	// Delete Labels and related objects
	if err := issues_model.DeleteLabelsByRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete Pulls and related objects
	if err := issues_model.DeletePullsByBaseRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete Issues and related objects
	var attachmentPaths []string
	if attachmentPaths, err = issues_model.DeleteIssuesByRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete issue index
	if err := db.DeleteResourceIndex(ctx, "issue_index", repoID); err != nil {
		return err
	}

	if repo.IsFork {
		if _, err := db.Exec(ctx, "UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repo.ForkID); err != nil {
			return fmt.Errorf("decrease fork count: %w", err)
		}
	}

	if _, err := db.Exec(ctx, "UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", uid); err != nil {
		return err
	}

	if len(repo.Topics) > 0 {
		if err := repo_model.RemoveTopicsFromRepo(ctx, repo.ID); err != nil {
			return err
		}
	}

	if err := project_model.DeleteProjectByRepoID(ctx, repoID); err != nil {
		return fmt.Errorf("unable to delete projects for repo[%d]: %w", repoID, err)
	}

	// Remove LFS objects
	var lfsObjects []*git_model.LFSMetaObject
	if err = sess.Where("repository_id=?", repoID).Find(&lfsObjects); err != nil {
		return err
	}

	lfsPaths := make([]string, 0, len(lfsObjects))
	for _, v := range lfsObjects {
		count, err := db.CountByBean(ctx, &git_model.LFSMetaObject{Pointer: lfs.Pointer{Oid: v.Oid}})
		if err != nil {
			return err
		}
		if count > 1 {
			continue
		}

		lfsPaths = append(lfsPaths, v.RelativePath())
	}

	if _, err := db.DeleteByBean(ctx, &git_model.LFSMetaObject{RepositoryID: repoID}); err != nil {
		return err
	}

	// Remove archives
	var archives []*repo_model.RepoArchiver
	if err = sess.Where("repo_id=?", repoID).Find(&archives); err != nil {
		return err
	}

	archivePaths := make([]string, 0, len(archives))
	for _, v := range archives {
		archivePaths = append(archivePaths, v.RelativePath())
	}

	if _, err := db.DeleteByBean(ctx, &repo_model.RepoArchiver{RepoID: repoID}); err != nil {
		return err
	}

	if repo.NumForks > 0 {
		if _, err = sess.Exec("UPDATE `repository` SET fork_id=0,is_fork=? WHERE fork_id=?", false, repo.ID); err != nil {
			log.Error("reset 'fork_id' and 'is_fork': %v", err)
		}
	}

	// Get all attachments with both issue_id and release_id are zero
	var newAttachments []*repo_model.Attachment
	if err := sess.Where(builder.Eq{
		"repo_id":    repo.ID,
		"issue_id":   0,
		"release_id": 0,
	}).Find(&newAttachments); err != nil {
		return err
	}

	newAttachmentPaths := make([]string, 0, len(newAttachments))
	for _, attach := range newAttachments {
		newAttachmentPaths = append(newAttachmentPaths, attach.RelativePath())
	}

	if _, err := sess.Where("repo_id=?", repo.ID).Delete(new(repo_model.Attachment)); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return err
	}

	committer.Close()

	if needRewriteKeysFile {
		if err := asymkey_model.RewriteAllPublicKeys(); err != nil {
			log.Error("RewriteAllPublicKeys failed: %v", err)
		}
	}

	// We should always delete the files after the database transaction succeed. If
	// we delete the file but the database rollback, the repository will be broken.

	// Remove repository files.
	repoPath := repo.RepoPath()
	system_model.RemoveAllWithNotice(db.DefaultContext, "Delete repository files", repoPath)

	// Remove wiki files
	if repo.HasWiki() {
		system_model.RemoveAllWithNotice(db.DefaultContext, "Delete repository wiki", repo.WikiPath())
	}

	// Remove archives
	for _, archive := range archivePaths {
		system_model.RemoveStorageWithNotice(db.DefaultContext, storage.RepoArchives, "Delete repo archive file", archive)
	}

	// Remove lfs objects
	for _, lfsObj := range lfsPaths {
		system_model.RemoveStorageWithNotice(db.DefaultContext, storage.LFS, "Delete orphaned LFS file", lfsObj)
	}

	// Remove issue attachment files.
	for _, attachment := range attachmentPaths {
		system_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete issue attachment", attachment)
	}

	// Remove release attachment files.
	for _, releaseAttachment := range releaseAttachments {
		system_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete release attachment", releaseAttachment)
	}

	// Remove attachment with no issue_id and release_id.
	for _, newAttachment := range newAttachmentPaths {
		system_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete issue attachment", newAttachment)
	}

	if len(repo.Avatar) > 0 {
		if err := storage.RepoAvatars.Delete(repo.CustomAvatarRelativePath()); err != nil {
			return fmt.Errorf("Failed to remove %s: %w", repo.Avatar, err)
		}
	}

	return nil
}
