// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

// ActionType represents the type of an action.
type ActionType int

// Possible action types.
const (
	ActionCreateRepo                ActionType = iota + 1 // 1
	ActionRenameRepo                                      // 2
	ActionStarRepo                                        // 3
	ActionWatchRepo                                       // 4
	ActionCommitRepo                                      // 5
	ActionCreateIssue                                     // 6
	ActionCreatePullRequest                               // 7
	ActionTransferRepo                                    // 8
	ActionPushTag                                         // 9
	ActionCommentIssue                                    // 10
	ActionMergePullRequest                                // 11
	ActionCloseIssue                                      // 12
	ActionReopenIssue                                     // 13
	ActionClosePullRequest                                // 14
	ActionReopenPullRequest                               // 15
	ActionDeleteTag                                       // 16
	ActionDeleteBranch                                    // 17
	ActionMirrorSyncPush                                  // 18
	ActionMirrorSyncCreate                                // 19
	ActionMirrorSyncDelete                                // 20
	ActionApprovePullRequest                              // 21
	ActionRejectPullRequest                               // 22
	ActionCommentPull                                     // 23
	ActionPublishRelease                                  // 24
	ActionPullReviewDismissed                             // 25
	ActionPullRequestReadyForReview                       // 26
	ActionAutoMergePullRequest                            // 27
)

func (at ActionType) String() string {
	switch at {
	case ActionCreateRepo:
		return "create_repo"
	case ActionRenameRepo:
		return "rename_repo"
	case ActionStarRepo:
		return "star_repo"
	case ActionWatchRepo:
		return "watch_repo"
	case ActionCommitRepo:
		return "commit_repo"
	case ActionCreateIssue:
		return "create_issue"
	case ActionCreatePullRequest:
		return "create_pull_request"
	case ActionTransferRepo:
		return "transfer_repo"
	case ActionPushTag:
		return "push_tag"
	case ActionCommentIssue:
		return "comment_issue"
	case ActionMergePullRequest:
		return "merge_pull_request"
	case ActionCloseIssue:
		return "close_issue"
	case ActionReopenIssue:
		return "reopen_issue"
	case ActionClosePullRequest:
		return "close_pull_request"
	case ActionReopenPullRequest:
		return "reopen_pull_request"
	case ActionDeleteTag:
		return "delete_tag"
	case ActionDeleteBranch:
		return "delete_branch"
	case ActionMirrorSyncPush:
		return "mirror_sync_push"
	case ActionMirrorSyncCreate:
		return "mirror_sync_create"
	case ActionMirrorSyncDelete:
		return "mirror_sync_delete"
	case ActionApprovePullRequest:
		return "approve_pull_request"
	case ActionRejectPullRequest:
		return "reject_pull_request"
	case ActionCommentPull:
		return "comment_pull"
	case ActionPublishRelease:
		return "publish_release"
	case ActionPullReviewDismissed:
		return "pull_review_dismissed"
	case ActionPullRequestReadyForReview:
		return "pull_request_ready_for_review"
	case ActionAutoMergePullRequest:
		return "auto_merge_pull_request"
	default:
		return "action-" + strconv.Itoa(int(at))
	}
}

func (at ActionType) InActions(actions ...string) bool {
	for _, action := range actions {
		if action == at.String() {
			return true
		}
	}
	return false
}

// Action represents user operation type and other information to
// repository. It implemented interface base.Actioner so that can be
// used in template render.
type Action struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64 `xorm:"INDEX"` // Receiver user id.
	OpType      ActionType
	ActUserID   int64            // Action user id.
	ActUser     *user_model.User `xorm:"-"`
	RepoID      int64
	Repo        *repo_model.Repository `xorm:"-"`
	CommentID   int64                  `xorm:"INDEX"`
	Comment     *issues_model.Comment  `xorm:"-"`
	Issue       *issues_model.Issue    `xorm:"-"` // get the issue id from content
	IsDeleted   bool                   `xorm:"NOT NULL DEFAULT false"`
	RefName     string
	IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
	Content     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(Action))
}

// TableIndices implements xorm's TableIndices interface
func (a *Action) TableIndices() []*schemas.Index {
	repoIndex := schemas.NewIndex("r_u_d", schemas.IndexType)
	repoIndex.AddColumn("repo_id", "user_id", "is_deleted")

	actUserIndex := schemas.NewIndex("au_r_c_u_d", schemas.IndexType)
	actUserIndex.AddColumn("act_user_id", "repo_id", "created_unix", "user_id", "is_deleted")

	cudIndex := schemas.NewIndex("c_u_d", schemas.IndexType)
	cudIndex.AddColumn("created_unix", "user_id", "is_deleted")

	indices := []*schemas.Index{actUserIndex, repoIndex, cudIndex}

	return indices
}

// ActivityReadable return whether doer can read activities of user
func ActivityReadable(user, doer *user_model.User) bool {
	return !user.KeepActivityPrivate ||
		doer != nil && (doer.IsAdmin || user.ID == doer.ID)
}

func activityQueryCondition(ctx context.Context, opts GetFeedsOptions) (builder.Cond, error) {
	cond := builder.NewCond()

	if opts.RequestedTeam != nil && opts.RequestedUser == nil {
		org, err := user_model.GetUserByID(ctx, opts.RequestedTeam.OrgID)
		if err != nil {
			return nil, err
		}
		opts.RequestedUser = org
	}

	// check activity visibility for actor ( similar to activityReadable() )
	if opts.Actor == nil {
		cond = cond.And(builder.In("act_user_id",
			builder.Select("`user`.id").Where(
				builder.Eq{"keep_activity_private": false, "visibility": structs.VisibleTypePublic},
			).From("`user`"),
		))
	} else if !opts.Actor.IsAdmin {
		uidCond := builder.Select("`user`.id").From("`user`").Where(
			builder.Eq{"keep_activity_private": false}.
				And(builder.In("visibility", structs.VisibleTypePublic, structs.VisibleTypeLimited))).
			Or(builder.Eq{"id": opts.Actor.ID})

		if opts.RequestedUser != nil {
			if opts.RequestedUser.IsOrganization() {
				// An organization can always see the activities whose `act_user_id` is the same as its id.
				uidCond = uidCond.Or(builder.Eq{"id": opts.RequestedUser.ID})
			} else {
				// A user can always see the activities of the organizations to which the user belongs.
				uidCond = uidCond.Or(
					builder.Eq{"type": user_model.UserTypeOrganization}.
						And(builder.In("`user`.id", builder.Select("org_id").
							Where(builder.Eq{"uid": opts.RequestedUser.ID}).
							From("team_user"))),
				)
			}
		}

		cond = cond.And(builder.In("act_user_id", uidCond))
	}

	// check readable repositories by doer/actor
	if opts.Actor == nil || !opts.Actor.IsAdmin {
		cond = cond.And(builder.In("repo_id", repo_model.AccessibleRepoIDsQuery(opts.Actor)))
	}

	if opts.RequestedRepo != nil {
		// repo's actions could have duplicate items, see the comment of NotifyWatchers
		// so here we only filter the "original items", aka: user_id == act_user_id
		cond = cond.And(
			builder.Eq{"`action`.repo_id": opts.RequestedRepo.ID},
			builder.Expr("`action`.user_id = `action`.act_user_id"),
		)
	}

	if opts.RequestedTeam != nil {
		env := organization.OrgFromUser(opts.RequestedUser).AccessibleTeamReposEnv(ctx, opts.RequestedTeam)
		teamRepoIDs, err := env.RepoIDs(1, opts.RequestedUser.NumRepos)
		if err != nil {
			return nil, fmt.Errorf("GetTeamRepositories: %w", err)
		}
		cond = cond.And(builder.In("repo_id", teamRepoIDs))
	}

	if opts.RequestedUser != nil {
		cond = cond.And(builder.Eq{"user_id": opts.RequestedUser.ID})

		if opts.OnlyPerformedBy {
			cond = cond.And(builder.Eq{"act_user_id": opts.RequestedUser.ID})
		}
	}

	if !opts.IncludePrivate {
		cond = cond.And(builder.Eq{"`action`.is_private": false})
	}
	if !opts.IncludeDeleted {
		cond = cond.And(builder.Eq{"is_deleted": false})
	}

	if opts.Date != "" {
		dateLow, err := time.ParseInLocation("2006-01-02", opts.Date, setting.DefaultUILocation)
		if err != nil {
			log.Warn("Unable to parse %s, filter not applied: %v", opts.Date, err)
		} else {
			dateHigh := dateLow.Add(86399000000000) // 23h59m59s

			cond = cond.And(builder.Gte{"`action`.created_unix": dateLow.Unix()})
			cond = cond.And(builder.Lte{"`action`.created_unix": dateHigh.Unix()})
		}
	}

	return cond, nil
}

// DeleteIssueActions delete all actions related with issueID
func DeleteIssueActions(ctx context.Context, repoID, issueID, issueIndex int64) error {
	// delete actions assigned to this issue
	e := db.GetEngine(ctx)

	// MariaDB has a performance bug: https://jira.mariadb.org/browse/MDEV-16289
	// so here it uses "DELETE ... WHERE IN" with pre-queried IDs.
	var lastCommentID int64
	commentIDs := make([]int64, 0, db.DefaultMaxInSize)
	for {
		commentIDs = commentIDs[:0]
		err := e.Select("`id`").Table(&issues_model.Comment{}).
			Where(builder.Eq{"issue_id": issueID}).And("`id` > ?", lastCommentID).
			OrderBy("`id`").Limit(db.DefaultMaxInSize).
			Find(&commentIDs)
		if err != nil {
			return err
		} else if len(commentIDs) == 0 {
			break
		} else if _, err = db.GetEngine(ctx).In("comment_id", commentIDs).Delete(&Action{}); err != nil {
			return err
		}
		lastCommentID = commentIDs[len(commentIDs)-1]
	}

	_, err := e.Where("repo_id = ?", repoID).
		In("op_type", ActionCreateIssue, ActionCreatePullRequest).
		Where("content LIKE ?", strconv.FormatInt(issueIndex, 10)+"|%"). // "IssueIndex|content..."
		Delete(&Action{})
	return err
}
