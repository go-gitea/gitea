// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

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
		return "star_repo" // will not displayed in feeds.tmpl
	case ActionWatchRepo:
		return "watch_repo" // will not displayed in feeds.tmpl
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

	cuIndex := schemas.NewIndex("c_u", schemas.IndexType)
	cuIndex.AddColumn("user_id", "is_deleted")

	actUserUserIndex := schemas.NewIndex("au_c_u", schemas.IndexType)
	actUserUserIndex.AddColumn("act_user_id", "created_unix", "user_id")

	indices := []*schemas.Index{actUserIndex, repoIndex, cudIndex, cuIndex, actUserUserIndex}

	return indices
}

// GetOpType gets the ActionType of this action.
func (a *Action) GetOpType() ActionType {
	return a.OpType
}

// LoadActUser loads a.ActUser
func (a *Action) LoadActUser(ctx context.Context) {
	if a.ActUser != nil {
		return
	}
	var err error
	a.ActUser, err = user_model.GetUserByID(ctx, a.ActUserID)
	if err == nil {
		return
	} else if user_model.IsErrUserNotExist(err) {
		a.ActUser = user_model.NewGhostUser()
	} else {
		log.Error("GetUserByID(%d): %v", a.ActUserID, err)
	}
}

func (a *Action) LoadRepo(ctx context.Context) error {
	if a.Repo != nil {
		return nil
	}
	var err error
	a.Repo, err = repo_model.GetRepositoryByID(ctx, a.RepoID)
	return err
}

// GetActFullName gets the action's user full name.
func (a *Action) GetActFullName(ctx context.Context) string {
	a.LoadActUser(ctx)
	return a.ActUser.FullName
}

// GetActUserName gets the action's user name.
func (a *Action) GetActUserName(ctx context.Context) string {
	a.LoadActUser(ctx)
	return a.ActUser.Name
}

// ShortActUserName gets the action's user name trimmed to max 20
// chars.
func (a *Action) ShortActUserName(ctx context.Context) string {
	return util.EllipsisDisplayString(a.GetActUserName(ctx), 20)
}

// GetActDisplayName gets the action's display name based on DEFAULT_SHOW_FULL_NAME, or falls back to the username if it is blank.
func (a *Action) GetActDisplayName(ctx context.Context) string {
	if setting.UI.DefaultShowFullName {
		trimmedFullName := strings.TrimSpace(a.GetActFullName(ctx))
		if len(trimmedFullName) > 0 {
			return trimmedFullName
		}
	}
	return a.ShortActUserName(ctx)
}

// GetActDisplayNameTitle gets the action's display name used for the title (tooltip) based on DEFAULT_SHOW_FULL_NAME
func (a *Action) GetActDisplayNameTitle(ctx context.Context) string {
	if setting.UI.DefaultShowFullName {
		return a.ShortActUserName(ctx)
	}
	return a.GetActFullName(ctx)
}

// GetRepoUserName returns the name of the action repository owner.
func (a *Action) GetRepoUserName(ctx context.Context) string {
	_ = a.LoadRepo(ctx)
	if a.Repo == nil {
		return "(non-existing-repo)"
	}
	return a.Repo.OwnerName
}

// ShortRepoUserName returns the name of the action repository owner
// trimmed to max 20 chars.
func (a *Action) ShortRepoUserName(ctx context.Context) string {
	return util.EllipsisDisplayString(a.GetRepoUserName(ctx), 20)
}

// GetRepoName returns the name of the action repository.
func (a *Action) GetRepoName(ctx context.Context) string {
	_ = a.LoadRepo(ctx)
	if a.Repo == nil {
		return "(non-existing-repo)"
	}
	return a.Repo.Name
}

// ShortRepoName returns the name of the action repository
// trimmed to max 33 chars.
func (a *Action) ShortRepoName(ctx context.Context) string {
	return util.EllipsisDisplayString(a.GetRepoName(ctx), 33)
}

// GetRepoPath returns the virtual path to the action repository.
func (a *Action) GetRepoPath(ctx context.Context) string {
	return path.Join(a.GetRepoUserName(ctx), a.GetRepoName(ctx))
}

// ShortRepoPath returns the virtual path to the action repository
// trimmed to max 20 + 1 + 33 chars.
func (a *Action) ShortRepoPath(ctx context.Context) string {
	return path.Join(a.ShortRepoUserName(ctx), a.ShortRepoName(ctx))
}

// GetRepoLink returns relative link to action repository.
func (a *Action) GetRepoLink(ctx context.Context) string {
	// path.Join will skip empty strings
	return path.Join(setting.AppSubURL, "/", url.PathEscape(a.GetRepoUserName(ctx)), url.PathEscape(a.GetRepoName(ctx)))
}

// GetRepoAbsoluteLink returns the absolute link to action repository.
func (a *Action) GetRepoAbsoluteLink(ctx context.Context) string {
	return setting.AppURL + url.PathEscape(a.GetRepoUserName(ctx)) + "/" + url.PathEscape(a.GetRepoName(ctx))
}

func (a *Action) loadComment(ctx context.Context) (err error) {
	if a.CommentID == 0 || a.Comment != nil {
		return nil
	}
	a.Comment, err = issues_model.GetCommentByID(ctx, a.CommentID)
	return err
}

// GetCommentHTMLURL returns link to action comment.
func (a *Action) GetCommentHTMLURL(ctx context.Context) string {
	if a == nil {
		return "#"
	}
	_ = a.loadComment(ctx)
	if a.Comment != nil {
		return a.Comment.HTMLURL(ctx)
	}

	if err := a.LoadIssue(ctx); err != nil || a.Issue == nil {
		return "#"
	}
	if err := a.Issue.LoadRepo(ctx); err != nil {
		return "#"
	}

	return a.Issue.HTMLURL()
}

// GetCommentLink returns link to action comment.
func (a *Action) GetCommentLink(ctx context.Context) string {
	if a == nil {
		return "#"
	}
	_ = a.loadComment(ctx)
	if a.Comment != nil {
		return a.Comment.Link(ctx)
	}

	if err := a.LoadIssue(ctx); err != nil || a.Issue == nil {
		return "#"
	}
	if err := a.Issue.LoadRepo(ctx); err != nil {
		return "#"
	}

	return a.Issue.Link()
}

// GetBranch returns the action's repository branch.
func (a *Action) GetBranch() string {
	return strings.TrimPrefix(a.RefName, git.BranchPrefix)
}

// GetRefLink returns the action's ref link.
func (a *Action) GetRefLink(ctx context.Context) string {
	return a.GetRepoLink(ctx) + "/src/" + git.RefName(a.RefName).RefWebLinkPath()
}

// GetTag returns the action's repository tag.
func (a *Action) GetTag() string {
	return strings.TrimPrefix(a.RefName, git.TagPrefix)
}

// GetContent returns the action's content.
func (a *Action) GetContent() string {
	return a.Content
}

// GetCreate returns the action creation time.
func (a *Action) GetCreate() time.Time {
	return a.CreatedUnix.AsTime()
}

func (a *Action) IsIssueEvent() bool {
	return a.OpType.InActions("comment_issue", "approve_pull_request", "reject_pull_request", "comment_pull", "merge_pull_request")
}

// GetIssueInfos returns a list of associated information with the action.
func (a *Action) GetIssueInfos() []string {
	// make sure it always returns 3 elements, because there are some access to the a[1] and a[2] without checking the length
	ret := strings.SplitN(a.Content, "|", 3)
	for len(ret) < 3 {
		ret = append(ret, "")
	}
	return ret
}

func (a *Action) getIssueIndex() int64 {
	infos := a.GetIssueInfos()
	if len(infos) == 0 {
		return 0
	}
	index, _ := strconv.ParseInt(infos[0], 10, 64)
	return index
}

func (a *Action) LoadIssue(ctx context.Context) error {
	if a.Issue != nil {
		return nil
	}
	if index := a.getIssueIndex(); index > 0 {
		issue, err := issues_model.GetIssueByIndex(ctx, a.RepoID, index)
		if err != nil {
			return err
		}
		a.Issue = issue
		a.Issue.Repo = a.Repo
	}
	return nil
}

// GetIssueTitle returns the title of first issue associated with the action.
func (a *Action) GetIssueTitle(ctx context.Context) string {
	if err := a.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return "<500 when get issue>"
	}
	if a.Issue == nil {
		return "<Issue not found>"
	}
	return a.Issue.Title
}

// GetIssueContent returns the content of first issue associated with this action.
func (a *Action) GetIssueContent(ctx context.Context) string {
	if err := a.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return "<500 when get issue>"
	}
	if a.Issue == nil {
		return "<Content not found>"
	}
	return a.Issue.Content
}

// GetFeedsOptions options for retrieving feeds
type GetFeedsOptions struct {
	db.ListOptions
	RequestedUser   *user_model.User       // the user we want activity for
	RequestedTeam   *organization.Team     // the team we want activity for
	RequestedRepo   *repo_model.Repository // the repo we want activity for
	Actor           *user_model.User       // the user viewing the activity
	IncludePrivate  bool                   // include private actions
	OnlyPerformedBy bool                   // only actions performed by requested user
	IncludeDeleted  bool                   // include deleted actions
	Date            string                 // the day we want activity for: YYYY-MM-DD
	DontCount       bool                   // do counting in GetFeeds
}

// ActivityReadable return whether doer can read activities of user
func ActivityReadable(user, doer *user_model.User) bool {
	return !user.KeepActivityPrivate ||
		doer != nil && (doer.IsAdmin || user.ID == doer.ID)
}

func FeedDateCond(opts GetFeedsOptions) builder.Cond {
	cond := builder.NewCond()
	if opts.Date == "" {
		return cond
	}

	dateLow, err := time.ParseInLocation("2006-01-02", opts.Date, setting.DefaultUILocation)
	if err != nil {
		log.Warn("Unable to parse %s, filter not applied: %v", opts.Date, err)
	} else {
		dateHigh := dateLow.Add(86399000000000) // 23h59m59s

		cond = cond.And(builder.Gte{"`action`.created_unix": dateLow.Unix()})
		cond = cond.And(builder.Lte{"`action`.created_unix": dateHigh.Unix()})
	}
	return cond
}

func ActivityQueryCondition(ctx context.Context, opts GetFeedsOptions) (builder.Cond, error) {
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
		env := repo_model.AccessibleTeamReposEnv(organization.OrgFromUser(opts.RequestedUser), opts.RequestedTeam)
		teamRepoIDs, err := env.RepoIDs(ctx)
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

	cond = cond.And(FeedDateCond(opts))

	return cond, nil
}

// DeleteOldActions deletes all old actions from database.
func DeleteOldActions(ctx context.Context, olderThan time.Duration) (err error) {
	if olderThan <= 0 {
		return nil
	}

	_, err = db.GetEngine(ctx).Where("created_unix < ?", time.Now().Add(-olderThan).Unix()).Delete(&Action{})
	return err
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

// CountActionCreatedUnixString count actions where created_unix is an empty string
func CountActionCreatedUnixString(ctx context.Context) (int64, error) {
	if setting.Database.Type.IsSQLite3() {
		return db.GetEngine(ctx).Where(`created_unix = ''`).Count(new(Action))
	}
	return 0, nil
}

// FixActionCreatedUnixString set created_unix to zero if it is an empty string
func FixActionCreatedUnixString(ctx context.Context) (int64, error) {
	if setting.Database.Type.IsSQLite3() {
		res, err := db.GetEngine(ctx).Exec(`UPDATE action SET created_unix = 0 WHERE created_unix = ''`)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	return 0, nil
}
