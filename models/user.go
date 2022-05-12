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
	"code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

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

	if err = db.DeleteBeans(ctx,
		&AccessToken{UID: u.ID},
		&repo_model.Collaboration{UserID: u.ID},
		&access_model.Access{UserID: u.ID},
		&repo_model.Watch{UserID: u.ID},
		&repo_model.Star{UID: u.ID},
		&user_model.Follow{UserID: u.ID},
		&user_model.Follow{FollowID: u.ID},
		&Action{UserID: u.ID},
		&IssueUser{UID: u.ID},
		&user_model.EmailAddress{UID: u.ID},
		&user_model.UserOpenID{UID: u.ID},
		&issues.Reaction{UserID: u.ID},
		&organization.TeamUser{UID: u.ID},
		&Stopwatch{UserID: u.ID},
		&user_model.Setting{UserID: u.ID},
		&pull_model.AutoMerge{DoerID: u.ID},
		&pull_model.ReviewState{UserID: u.ID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	if err := auth_model.DeleteOAuth2RelictsByUserID(ctx, u.ID); err != nil {
		return err
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
				if err = deleteComment(ctx, comment); err != nil {
					return err
				}
			}
		}

		// Delete Reactions
		if err = issues.DeleteReaction(ctx, &issues.ReactionOptions{DoerID: u.ID}); err != nil {
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
