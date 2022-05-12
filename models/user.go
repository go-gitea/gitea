// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"time"

	_ "image/jpeg" // Needed for jpeg support

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// GetOrganizationCount returns count of membership of organization of the user.
func GetOrganizationCount(ctx context.Context, u *user_model.User) (int64, error) {
	return db.GetEngine(ctx).
		Where("uid=?", u.ID).
		Count(new(OrgUser))
}

// deleteBeans deletes all given beans, beans should contain delete conditions.
func deleteBeans(e db.Engine, beans ...interface{}) (err error) {
	for i := range beans {
		if _, err = e.Delete(beans[i]); err != nil {
			return err
		}
	}
	return nil
}

// DeleteUser deletes models associated to an user.
func DeleteUser(ctx context.Context, u *user_model.User) (err error) {
	e := db.GetEngine(ctx)

	// ***** START: Watch *****
	watchedRepoIDs := make([]int64, 0, 10)
	if err = e.Table("watch").Cols("watch.repo_id").
		Where("watch.user_id = ?", u.ID).And("watch.mode <>?", repo_model.WatchModeDont).Find(&watchedRepoIDs); err != nil {
		return fmt.Errorf("get all watches: %v", err)
	}
	if _, err = e.Decr("num_watches").In("id", watchedRepoIDs).NoAutoTime().Update(new(repo_model.Repository)); err != nil {
		return fmt.Errorf("decrease repository num_watches: %v", err)
	}
	// ***** END: Watch *****

	// ***** START: Star *****
	starredRepoIDs := make([]int64, 0, 10)
	if err = e.Table("star").Cols("star.repo_id").
		Where("star.uid = ?", u.ID).Find(&starredRepoIDs); err != nil {
		return fmt.Errorf("get all stars: %v", err)
	} else if _, err = e.Decr("num_stars").In("id", starredRepoIDs).NoAutoTime().Update(new(repo_model.Repository)); err != nil {
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

	if _, err := db.GetEngine(ctx).In("grant_id", builder.Select("id").From("oauth2_grant").Where(builder.Eq{"oauth2_grant.user_id": u.ID})).
		Delete(&auth_model.OAuth2AuthorizationCode{}); err != nil {
		return err
	}

	if err = deleteBeans(e,
		&AccessToken{UID: u.ID},
		&Collaboration{UserID: u.ID},
		&Access{UserID: u.ID},
		&repo_model.Watch{UserID: u.ID},
		&repo_model.Star{UID: u.ID},
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
		&user_model.Setting{UserID: u.ID},
		&auth_model.OAuth2Application{UID: u.ID},
		&auth_model.OAuth2Grant{UserID: u.ID},
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

	// ***** START: Branch Protections *****
	{
		const batchSize = 50
		for start := 0; ; start += batchSize {
			protections := make([]*ProtectedBranch, 0, batchSize)
			// @perf: We can't filter on DB side by u.ID, as those IDs are serialized as JSON strings.
			//   We could filter down with `WHERE repo_id IN (reposWithPushPermission(u))`,
			//   though that query will be quite complex and tricky to maintain (compare `getRepoAssignees()`).
			// Also, as we didn't update branch protections when removing entries from `access` table,
			//   it's safer to iterate all protected branches.
			if err = e.Limit(batchSize, start).Find(&protections); err != nil {
				return fmt.Errorf("findProtectedBranches: %v", err)
			}
			if len(protections) == 0 {
				break
			}
			for _, p := range protections {
				var matched1, matched2, matched3 bool
				if len(p.WhitelistUserIDs) != 0 {
					p.WhitelistUserIDs, matched1 = util.RemoveIDFromList(
						p.WhitelistUserIDs, u.ID)
				}
				if len(p.ApprovalsWhitelistUserIDs) != 0 {
					p.ApprovalsWhitelistUserIDs, matched2 = util.RemoveIDFromList(
						p.ApprovalsWhitelistUserIDs, u.ID)
				}
				if len(p.MergeWhitelistUserIDs) != 0 {
					p.MergeWhitelistUserIDs, matched3 = util.RemoveIDFromList(
						p.MergeWhitelistUserIDs, u.ID)
				}
				if matched1 || matched2 || matched3 {
					if _, err = e.ID(p.ID).Cols(
						"whitelist_user_i_ds",
						"merge_whitelist_user_i_ds",
						"approvals_whitelist_user_i_ds",
					).Update(p); err != nil {
						return fmt.Errorf("updateProtectedBranches: %v", err)
					}
				}
			}
		}
	}
	// ***** END: Branch Protections *****

	// ***** START: PublicKey *****
	if _, err = e.Delete(&asymkey_model.PublicKey{OwnerID: u.ID}); err != nil {
		return fmt.Errorf("deletePublicKeys: %v", err)
	}
	// ***** END: PublicKey *****

	// ***** START: GPGPublicKey *****
	keys, err := asymkey_model.ListGPGKeys(ctx, u.ID, db.ListOptions{})
	if err != nil {
		return fmt.Errorf("ListGPGKeys: %v", err)
	}
	// Delete GPGKeyImport(s).
	for _, key := range keys {
		if _, err = e.Delete(&asymkey_model.GPGKeyImport{KeyID: key.KeyID}); err != nil {
			return fmt.Errorf("deleteGPGKeyImports: %v", err)
		}
	}
	if _, err = e.Delete(&asymkey_model.GPGKey{OwnerID: u.ID}); err != nil {
		return fmt.Errorf("deleteGPGKeys: %v", err)
	}
	// ***** END: GPGPublicKey *****

	// Clear assignee.
	if err = clearAssigneeByUserID(e, u.ID); err != nil {
		return fmt.Errorf("clear assignee: %v", err)
	}

	// ***** START: ExternalLoginUser *****
	if err = user_model.RemoveAllAccountLinks(ctx, u); err != nil {
		return fmt.Errorf("ExternalLoginUser: %v", err)
	}
	// ***** END: ExternalLoginUser *****

	if _, err = e.ID(u.ID).Delete(new(user_model.User)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	return nil
}

// GetStarredRepos returns the repos starred by a particular user
func GetStarredRepos(userID int64, private bool, listOptions db.ListOptions) ([]*repo_model.Repository, error) {
	sess := db.GetEngine(db.DefaultContext).Where("star.uid=?", userID).
		Join("LEFT", "star", "`repository`.id=`star`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)

		repos := make([]*repo_model.Repository, 0, listOptions.PageSize)
		return repos, sess.Find(&repos)
	}

	repos := make([]*repo_model.Repository, 0, 10)
	return repos, sess.Find(&repos)
}

// GetWatchedRepos returns the repos watched by a particular user
func GetWatchedRepos(userID int64, private bool, listOptions db.ListOptions) ([]*repo_model.Repository, int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where("watch.user_id=?", userID).
		And("`watch`.mode<>?", repo_model.WatchModeDont).
		Join("LEFT", "watch", "`repository`.id=`watch`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)

		repos := make([]*repo_model.Repository, 0, listOptions.PageSize)
		total, err := sess.FindAndCount(&repos)
		return repos, total, err
	}

	repos := make([]*repo_model.Repository, 0, 10)
	total, err := sess.FindAndCount(&repos)
	return repos, total, err
}

// IsUserVisibleToViewer check if viewer is able to see user profile
func IsUserVisibleToViewer(u, viewer *user_model.User) bool {
	return isUserVisibleToViewer(db.GetEngine(db.DefaultContext), u, viewer)
}

func isUserVisibleToViewer(e db.Engine, u, viewer *user_model.User) bool {
	if viewer != nil && viewer.IsAdmin {
		return true
	}

	switch u.Visibility {
	case structs.VisibleTypePublic:
		return true
	case structs.VisibleTypeLimited:
		if viewer == nil || viewer.IsRestricted {
			return false
		}
		return true
	case structs.VisibleTypePrivate:
		if viewer == nil || viewer.IsRestricted {
			return false
		}

		// If they follow - they see each over
		follower := user_model.IsFollowing(u.ID, viewer.ID)
		if follower {
			return true
		}

		// Now we need to check if they in some organization together
		count, err := e.Table("team_user").
			Where(
				builder.And(
					builder.Eq{"uid": viewer.ID},
					builder.Or(
						builder.Eq{"org_id": u.ID},
						builder.In("org_id",
							builder.Select("org_id").
								From("team_user", "t2").
								Where(builder.Eq{"uid": u.ID}))))).
			Count(new(TeamUser))
		if err != nil {
			return false
		}

		if count < 0 {
			// No common organization
			return false
		}

		// they are in an organization together
		return true
	}
	return false
}
