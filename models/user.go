// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
)

// deleteBeans deletes all given beans, beans should contain delete conditions.
func deleteBeans(e db.Engine, beans ...interface{}) (err error) {
	for i := range beans {
		if _, err = e.Delete(beans[i]); err != nil {
			return err
		}
	}
	return nil
}

func deleteUser(ctx context.Context, u *user_model.User) error {
	e := db.GetEngine(ctx)

	// Note: A user owns any repository or belongs to any organization
	//	cannot perform delete operation.

	// Check ownership of repository.
	count, err := getRepositoryCount(e, u.ID)
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %v", err)
	} else if count > 0 {
		return ErrUserOwnRepos{UID: u.ID}
	}

	// Check membership of organization.
	count, err = u.GetOrganizationCount(ctx)
	if err != nil {
		return fmt.Errorf("GetOrganizationCount: %v", err)
	} else if count > 0 {
		return ErrUserHasOrgs{UID: u.ID}
	}

	// ***** START: Watch *****
	watchedRepoIDs := make([]int64, 0, 10)
	if err = e.Table("watch").Cols("watch.repo_id").
		Where("watch.user_id = ?", u.ID).And("watch.mode <>?", RepoWatchModeDont).Find(&watchedRepoIDs); err != nil {
		return fmt.Errorf("get all watches: %v", err)
	}
	if _, err = e.Decr("num_watches").In("id", watchedRepoIDs).NoAutoTime().Update(new(Repository)); err != nil {
		return fmt.Errorf("decrease repository num_watches: %v", err)
	}
	// ***** END: Watch *****

	// ***** START: Star *****
	starredRepoIDs := make([]int64, 0, 10)
	if err = e.Table("star").Cols("star.repo_id").
		Where("star.uid = ?", u.ID).Find(&starredRepoIDs); err != nil {
		return fmt.Errorf("get all stars: %v", err)
	} else if _, err = e.Decr("num_stars").In("id", starredRepoIDs).NoAutoTime().Update(new(Repository)); err != nil {
		return fmt.Errorf("decrease repository num_stars: %v", err)
	}
	// ***** END: Star *****

	// ***** START: Follow *****
	followeeIDs := make([]int64, 0, 10)
	if err = e.Table("follow").Cols("follow.follow_id").
		Where("follow.user_id = ?", u.ID).Find(&followeeIDs); err != nil {
		return fmt.Errorf("get all followees: %v", err)
	} else if _, err = e.Decr("num_followers").In("id", followeeIDs).Update(new(user_model.User)); err != nil {
		return fmt.Errorf("decrease user num_followers: %v", err)
	}

	followerIDs := make([]int64, 0, 10)
	if err = e.Table("follow").Cols("follow.user_id").
		Where("follow.follow_id = ?", u.ID).Find(&followerIDs); err != nil {
		return fmt.Errorf("get all followers: %v", err)
	} else if _, err = e.Decr("num_following").In("id", followerIDs).Update(new(user_model.User)); err != nil {
		return fmt.Errorf("decrease user num_following: %v", err)
	}
	// ***** END: Follow *****

	if err = deleteBeans(e,
		&AccessToken{UID: u.ID},
		&Collaboration{UserID: u.ID},
		&Access{UserID: u.ID},
		&Watch{UserID: u.ID},
		&Star{UID: u.ID},
		&user_model.Follow{UserID: u.ID},
		&user_model.Follow{FollowID: u.ID},
		&Action{UserID: u.ID},
		&IssueUser{UID: u.ID},
		&user_model.EmailAddress{UID: u.ID},
		&user_model.UserOpenID{UID: u.ID},
		&Reaction{UserID: u.ID},
		&TeamUser{UID: u.ID},
		&Collaboration{UserID: u.ID},
		&Stopwatch{UserID: u.ID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	if setting.Service.UserDeleteWithCommentsMaxTime != 0 &&
		u.CreatedUnix.AsTime().Add(setting.Service.UserDeleteWithCommentsMaxTime).After(time.Now()) {

		// Delete Comments
		const batchSize = 50
		for start := 0; ; start += batchSize {
			comments := make([]*Comment, 0, batchSize)
			if err = e.Where("type=? AND poster_id=?", CommentTypeComment, u.ID).Limit(batchSize, start).Find(&comments); err != nil {
				return err
			}
			if len(comments) == 0 {
				break
			}

			for _, comment := range comments {
				if err = deleteComment(e, comment); err != nil {
					return err
				}
			}
		}

		// Delete Reactions
		if err = deleteReaction(e, &ReactionOptions{Doer: u}); err != nil {
			return err
		}
	}

	// ***** START: PublicKey *****
	if _, err = e.Delete(&PublicKey{OwnerID: u.ID}); err != nil {
		return fmt.Errorf("deletePublicKeys: %v", err)
	}
	err = rewriteAllPublicKeys(e)
	if err != nil {
		return err
	}
	err = rewriteAllPrincipalKeys(e)
	if err != nil {
		return err
	}
	// ***** END: PublicKey *****

	// ***** START: GPGPublicKey *****
	keys, err := listGPGKeys(e, u.ID, db.ListOptions{})
	if err != nil {
		return fmt.Errorf("ListGPGKeys: %v", err)
	}
	// Delete GPGKeyImport(s).
	for _, key := range keys {
		if _, err = e.Delete(&GPGKeyImport{KeyID: key.KeyID}); err != nil {
			return fmt.Errorf("deleteGPGKeyImports: %v", err)
		}
	}
	if _, err = e.Delete(&GPGKey{OwnerID: u.ID}); err != nil {
		return fmt.Errorf("deleteGPGKeys: %v", err)
	}
	// ***** END: GPGPublicKey *****

	// Clear assignee.
	if err = clearAssigneeByUserID(e, u.ID); err != nil {
		return fmt.Errorf("clear assignee: %v", err)
	}

	// ***** START: ExternalLoginUser *****
	if err = removeAllAccountLinks(e, u); err != nil {
		return fmt.Errorf("ExternalLoginUser: %v", err)
	}
	// ***** END: ExternalLoginUser *****

	if _, err = e.ID(u.ID).Delete(new(user_model.User)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := user_model.UserPath(u.Name)
	if err = util.RemoveAll(path); err != nil {
		err = fmt.Errorf("Failed to RemoveAll %s: %v", path, err)
		_ = createNotice(e, NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
		return err
	}

	if len(u.Avatar) > 0 {
		avatarPath := u.CustomAvatarRelativePath()
		if err = storage.Avatars.Delete(avatarPath); err != nil {
			err = fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			_ = createNotice(e, NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
			return err
		}
	}

	return nil
}

// DeleteUser completely and permanently deletes everything of a user,
// but issues/comments/pulls will be kept and shown as someone has been deleted,
// unless the user is younger than USER_DELETE_WITH_COMMENTS_MAX_DAYS.
func DeleteUser(u *user_model.User) (err error) {
	if u.IsOrganization() {
		return fmt.Errorf("%s is an organization not a user", u.Name)
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = deleteUser(ctx, u); err != nil {
		// Note: don't wrapper error here.
		return err
	}

	return committer.Commit()
}

// DeleteInactiveUsers deletes all inactive users and email addresses.
func DeleteInactiveUsers(ctx context.Context, olderThan time.Duration) (err error) {
	users := make([]*user_model.User, 0, 10)
	if olderThan > 0 {
		if err = db.GetEngine(db.DefaultContext).
			Where("is_active = ? and created_unix < ?", false, time.Now().Add(-olderThan).Unix()).
			Find(&users); err != nil {
			return fmt.Errorf("get all inactive users: %v", err)
		}
	} else {
		if err = db.GetEngine(db.DefaultContext).
			Where("is_active = ?", false).
			Find(&users); err != nil {
			return fmt.Errorf("get all inactive users: %v", err)
		}
	}
	// FIXME: should only update authorized_keys file once after all deletions.
	for _, u := range users {
		select {
		case <-ctx.Done():
			return db.ErrCancelledf("Before delete inactive user %s", u.Name)
		default:
		}
		if err = DeleteUser(u); err != nil {
			// Ignore users that were set inactive by admin.
			if IsErrUserOwnRepos(err) || IsErrUserHasOrgs(err) {
				continue
			}
			return err
		}
	}

	_, err = db.GetEngine(db.DefaultContext).
		Where("is_activated = ?", false).
		Delete(new(user_model.EmailAddress))
	return err
}

// GetWatchedRepos returns the repos watched by a particular user
func GetWatchedRepos(userID int64, private bool, listOptions db.ListOptions) ([]*Repository, int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where("watch.user_id=?", userID).
		And("`watch`.mode<>?", RepoWatchModeDont).
		Join("LEFT", "watch", "`repository`.id=`watch`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)

		repos := make([]*Repository, 0, listOptions.PageSize)
		total, err := sess.FindAndCount(&repos)
		return repos, total, err
	}

	repos := make([]*Repository, 0, 10)
	total, err := sess.FindAndCount(&repos)
	return repos, total, err
}

// GetRepositories returns repositories that user owns, including private repositories.
func GetRepositories(u *user_model.User, listOpts db.ListOptions, names ...string) ([]*Repository, error) {
	repos, _, err := GetUserRepositories(&SearchRepoOptions{Actor: u, Private: true, ListOptions: listOpts, LowerNames: names})
	return repos, err
}
