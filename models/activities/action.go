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
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
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

// Action represents user operation type and other information to
// repository. It implemented interface base.Actioner so that can be
// used in template render.
type Action struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64 // Receiver user id.
	OpType      ActionType
	ActUserID   int64            // Action user id.
	ActUser     *user_model.User `xorm:"-"`
	RepoID      int64
	Repo        *repo_model.Repository `xorm:"-"`
	CommentID   int64                  `xorm:"INDEX"`
	Comment     *issues_model.Comment  `xorm:"-"`
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

func (a *Action) loadRepo(ctx context.Context) {
	if a.Repo != nil {
		return
	}
	var err error
	a.Repo, err = repo_model.GetRepositoryByID(ctx, a.RepoID)
	if err != nil {
		log.Error("repo_model.GetRepositoryByID(%d): %v", a.RepoID, err)
	}
}

// GetActFullName gets the action's user full name.
func (a *Action) GetActFullName() string {
	a.LoadActUser(db.DefaultContext)
	return a.ActUser.FullName
}

// GetActUserName gets the action's user name.
func (a *Action) GetActUserName() string {
	a.LoadActUser(db.DefaultContext)
	return a.ActUser.Name
}

// ShortActUserName gets the action's user name trimmed to max 20
// chars.
func (a *Action) ShortActUserName() string {
	return base.EllipsisString(a.GetActUserName(), 20)
}

// GetDisplayName gets the action's display name based on DEFAULT_SHOW_FULL_NAME, or falls back to the username if it is blank.
func (a *Action) GetDisplayName() string {
	if setting.UI.DefaultShowFullName {
		trimmedFullName := strings.TrimSpace(a.GetActFullName())
		if len(trimmedFullName) > 0 {
			return trimmedFullName
		}
	}
	return a.ShortActUserName()
}

// GetDisplayNameTitle gets the action's display name used for the title (tooltip) based on DEFAULT_SHOW_FULL_NAME
func (a *Action) GetDisplayNameTitle() string {
	if setting.UI.DefaultShowFullName {
		return a.ShortActUserName()
	}
	return a.GetActFullName()
}

// GetRepoUserName returns the name of the action repository owner.
func (a *Action) GetRepoUserName() string {
	a.loadRepo(db.DefaultContext)
	return a.Repo.OwnerName
}

// ShortRepoUserName returns the name of the action repository owner
// trimmed to max 20 chars.
func (a *Action) ShortRepoUserName() string {
	return base.EllipsisString(a.GetRepoUserName(), 20)
}

// GetRepoName returns the name of the action repository.
func (a *Action) GetRepoName() string {
	a.loadRepo(db.DefaultContext)
	return a.Repo.Name
}

// ShortRepoName returns the name of the action repository
// trimmed to max 33 chars.
func (a *Action) ShortRepoName() string {
	return base.EllipsisString(a.GetRepoName(), 33)
}

// GetRepoPath returns the virtual path to the action repository.
func (a *Action) GetRepoPath() string {
	return path.Join(a.GetRepoUserName(), a.GetRepoName())
}

// ShortRepoPath returns the virtual path to the action repository
// trimmed to max 20 + 1 + 33 chars.
func (a *Action) ShortRepoPath() string {
	return path.Join(a.ShortRepoUserName(), a.ShortRepoName())
}

// GetRepoLink returns relative link to action repository.
func (a *Action) GetRepoLink() string {
	// path.Join will skip empty strings
	return path.Join(setting.AppSubURL, "/", url.PathEscape(a.GetRepoUserName()), url.PathEscape(a.GetRepoName()))
}

// GetRepoAbsoluteLink returns the absolute link to action repository.
func (a *Action) GetRepoAbsoluteLink() string {
	return setting.AppURL + url.PathEscape(a.GetRepoUserName()) + "/" + url.PathEscape(a.GetRepoName())
}

// GetCommentHTMLURL returns link to action comment.
func (a *Action) GetCommentHTMLURL() string {
	return a.getCommentHTMLURL(db.DefaultContext)
}

func (a *Action) loadComment(ctx context.Context) (err error) {
	if a.CommentID == 0 || a.Comment != nil {
		return nil
	}
	a.Comment, err = issues_model.GetCommentByID(ctx, a.CommentID)
	return err
}

func (a *Action) getCommentHTMLURL(ctx context.Context) string {
	if a == nil {
		return "#"
	}
	_ = a.loadComment(ctx)
	if a.Comment != nil {
		return a.Comment.HTMLURL()
	}
	if len(a.GetIssueInfos()) == 0 {
		return "#"
	}
	// Return link to issue
	issueIDString := a.GetIssueInfos()[0]
	issueID, err := strconv.ParseInt(issueIDString, 10, 64)
	if err != nil {
		return "#"
	}

	issue, err := issues_model.GetIssueByID(ctx, issueID)
	if err != nil {
		return "#"
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return "#"
	}

	return issue.HTMLURL()
}

// GetCommentLink returns link to action comment.
func (a *Action) GetCommentLink() string {
	return a.getCommentLink(db.DefaultContext)
}

func (a *Action) getCommentLink(ctx context.Context) string {
	if a == nil {
		return "#"
	}
	_ = a.loadComment(ctx)
	if a.Comment != nil {
		return a.Comment.Link()
	}
	if len(a.GetIssueInfos()) == 0 {
		return "#"
	}
	// Return link to issue
	issueIDString := a.GetIssueInfos()[0]
	issueID, err := strconv.ParseInt(issueIDString, 10, 64)
	if err != nil {
		return "#"
	}

	issue, err := issues_model.GetIssueByID(ctx, issueID)
	if err != nil {
		return "#"
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return "#"
	}

	return issue.Link()
}

// GetBranch returns the action's repository branch.
func (a *Action) GetBranch() string {
	return strings.TrimPrefix(a.RefName, git.BranchPrefix)
}

// GetRefLink returns the action's ref link.
func (a *Action) GetRefLink() string {
	switch {
	case strings.HasPrefix(a.RefName, git.BranchPrefix):
		return a.GetRepoLink() + "/src/branch/" + util.PathEscapeSegments(strings.TrimPrefix(a.RefName, git.BranchPrefix))
	case strings.HasPrefix(a.RefName, git.TagPrefix):
		return a.GetRepoLink() + "/src/tag/" + util.PathEscapeSegments(strings.TrimPrefix(a.RefName, git.TagPrefix))
	case len(a.RefName) == git.SHAFullLength && git.IsValidSHAPattern(a.RefName):
		return a.GetRepoLink() + "/src/commit/" + a.RefName
	default:
		// FIXME: we will just assume it's a branch - this was the old way - at some point we may want to enforce that there is always a ref here.
		return a.GetRepoLink() + "/src/branch/" + util.PathEscapeSegments(strings.TrimPrefix(a.RefName, git.BranchPrefix))
	}
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

// GetIssueInfos returns a list of issues associated with
// the action.
func (a *Action) GetIssueInfos() []string {
	return strings.SplitN(a.Content, "|", 3)
}

// GetIssueTitle returns the title of first issue associated
// with the action.
func (a *Action) GetIssueTitle() string {
	index, _ := strconv.ParseInt(a.GetIssueInfos()[0], 10, 64)
	issue, err := issues_model.GetIssueByIndex(a.RepoID, index)
	if err != nil {
		log.Error("GetIssueByIndex: %v", err)
		return "500 when get issue"
	}
	return issue.Title
}

// GetIssueContent returns the content of first issue associated with
// this action.
func (a *Action) GetIssueContent() string {
	index, _ := strconv.ParseInt(a.GetIssueInfos()[0], 10, 64)
	issue, err := issues_model.GetIssueByIndex(a.RepoID, index)
	if err != nil {
		log.Error("GetIssueByIndex: %v", err)
		return "500 when get issue"
	}
	return issue.Content
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
}

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts GetFeedsOptions) (ActionList, int64, error) {
	if opts.RequestedUser == nil && opts.RequestedTeam == nil && opts.RequestedRepo == nil {
		return nil, 0, fmt.Errorf("need at least one of these filters: RequestedUser, RequestedTeam, RequestedRepo")
	}

	cond, err := activityQueryCondition(opts)
	if err != nil {
		return nil, 0, err
	}

	sess := db.GetEngine(ctx).Where(cond).
		Select("`action`.*"). // this line will avoid select other joined table's columns
		Join("INNER", "repository", "`repository`.id = `action`.repo_id")

	opts.SetDefaultValues()
	sess = db.SetSessionPagination(sess, &opts)

	actions := make([]*Action, 0, opts.PageSize)
	count, err := sess.Desc("`action`.created_unix").FindAndCount(&actions)
	if err != nil {
		return nil, 0, fmt.Errorf("FindAndCount: %w", err)
	}

	if err := ActionList(actions).loadAttributes(ctx); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %w", err)
	}

	return actions, count, nil
}

// ActivityReadable return whether doer can read activities of user
func ActivityReadable(user, doer *user_model.User) bool {
	return !user.KeepActivityPrivate ||
		doer != nil && (doer.IsAdmin || user.ID == doer.ID)
}

func activityQueryCondition(opts GetFeedsOptions) (builder.Cond, error) {
	cond := builder.NewCond()

	if opts.RequestedTeam != nil && opts.RequestedUser == nil {
		org, err := user_model.GetUserByID(db.DefaultContext, opts.RequestedTeam.OrgID)
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
		cond = cond.And(builder.In("act_user_id",
			builder.Select("`user`.id").Where(
				builder.Eq{"keep_activity_private": false}.
					And(builder.In("visibility", structs.VisibleTypePublic, structs.VisibleTypeLimited))).
				Or(builder.Eq{"id": opts.Actor.ID}).From("`user`"),
		))
	}

	// check readable repositories by doer/actor
	if opts.Actor == nil || !opts.Actor.IsAdmin {
		cond = cond.And(builder.In("repo_id", repo_model.AccessibleRepoIDsQuery(opts.Actor)))
	}

	if opts.RequestedRepo != nil {
		cond = cond.And(builder.Eq{"repo_id": opts.RequestedRepo.ID})
	}

	if opts.RequestedTeam != nil {
		env := organization.OrgFromUser(opts.RequestedUser).AccessibleTeamReposEnv(opts.RequestedTeam)
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

// DeleteOldActions deletes all old actions from database.
func DeleteOldActions(olderThan time.Duration) (err error) {
	if olderThan <= 0 {
		return nil
	}

	_, err = db.GetEngine(db.DefaultContext).Where("created_unix < ?", time.Now().Add(-olderThan).Unix()).Delete(&Action{})
	return err
}

// NotifyWatchers creates batch of actions for every watcher.
func NotifyWatchers(ctx context.Context, actions ...*Action) error {
	var watchers []*repo_model.Watch
	var repo *repo_model.Repository
	var err error
	var permCode []bool
	var permIssue []bool
	var permPR []bool

	e := db.GetEngine(ctx)

	for _, act := range actions {
		repoChanged := repo == nil || repo.ID != act.RepoID

		if repoChanged {
			// Add feeds for user self and all watchers.
			watchers, err = repo_model.GetWatchers(ctx, act.RepoID)
			if err != nil {
				return fmt.Errorf("get watchers: %w", err)
			}
		}

		// Add feed for actioner.
		act.UserID = act.ActUserID
		if _, err = e.Insert(act); err != nil {
			return fmt.Errorf("insert new actioner: %w", err)
		}

		if repoChanged {
			act.loadRepo(ctx)
			repo = act.Repo

			// check repo owner exist.
			if err := act.Repo.LoadOwner(ctx); err != nil {
				return fmt.Errorf("can't get repo owner: %w", err)
			}
		} else if act.Repo == nil {
			act.Repo = repo
		}

		// Add feed for organization
		if act.Repo.Owner.IsOrganization() && act.ActUserID != act.Repo.Owner.ID {
			act.ID = 0
			act.UserID = act.Repo.Owner.ID
			if err = db.Insert(ctx, act); err != nil {
				return fmt.Errorf("insert new actioner: %w", err)
			}
		}

		if repoChanged {
			permCode = make([]bool, len(watchers))
			permIssue = make([]bool, len(watchers))
			permPR = make([]bool, len(watchers))
			for i, watcher := range watchers {
				user, err := user_model.GetUserByID(ctx, watcher.UserID)
				if err != nil {
					permCode[i] = false
					permIssue[i] = false
					permPR[i] = false
					continue
				}
				perm, err := access_model.GetUserRepoPermission(ctx, repo, user)
				if err != nil {
					permCode[i] = false
					permIssue[i] = false
					permPR[i] = false
					continue
				}
				permCode[i] = perm.CanRead(unit.TypeCode)
				permIssue[i] = perm.CanRead(unit.TypeIssues)
				permPR[i] = perm.CanRead(unit.TypePullRequests)
			}
		}

		for i, watcher := range watchers {
			if act.ActUserID == watcher.UserID {
				continue
			}
			act.ID = 0
			act.UserID = watcher.UserID
			act.Repo.Units = nil

			switch act.OpType {
			case ActionCommitRepo, ActionPushTag, ActionDeleteTag, ActionPublishRelease, ActionDeleteBranch:
				if !permCode[i] {
					continue
				}
			case ActionCreateIssue, ActionCommentIssue, ActionCloseIssue, ActionReopenIssue:
				if !permIssue[i] {
					continue
				}
			case ActionCreatePullRequest, ActionCommentPull, ActionMergePullRequest, ActionClosePullRequest, ActionReopenPullRequest, ActionAutoMergePullRequest:
				if !permPR[i] {
					continue
				}
			}

			if err = db.Insert(ctx, act); err != nil {
				return fmt.Errorf("insert new action: %w", err)
			}
		}
	}
	return nil
}

// NotifyWatchersActions creates batch of actions for every watcher.
func NotifyWatchersActions(acts []*Action) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	for _, act := range acts {
		if err := NotifyWatchers(ctx, act); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// DeleteIssueActions delete all actions related with issueID
func DeleteIssueActions(ctx context.Context, repoID, issueID int64) error {
	// delete actions assigned to this issue
	subQuery := builder.Select("`id`").
		From("`comment`").
		Where(builder.Eq{"`issue_id`": issueID})
	if _, err := db.GetEngine(ctx).In("comment_id", subQuery).Delete(&Action{}); err != nil {
		return err
	}

	_, err := db.GetEngine(ctx).Table("action").Where("repo_id = ?", repoID).
		In("op_type", ActionCreateIssue, ActionCreatePullRequest).
		Where("content LIKE ?", strconv.FormatInt(issueID, 10)+"|%").
		Delete(&Action{})
	return err
}

// CountActionCreatedUnixString count actions where created_unix is an empty string
func CountActionCreatedUnixString(ctx context.Context) (int64, error) {
	if setting.Database.Type.IsSQLite3() {
		return db.GetEngine(ctx).Where(`created_unix = ""`).Count(new(Action))
	}
	return 0, nil
}

// FixActionCreatedUnixString set created_unix to zero if it is an empty string
func FixActionCreatedUnixString(ctx context.Context) (int64, error) {
	if setting.Database.Type.IsSQLite3() {
		res, err := db.GetEngine(ctx).Exec(`UPDATE action SET created_unix = 0 WHERE created_unix = ""`)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	return 0, nil
}
