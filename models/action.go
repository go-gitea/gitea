// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
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
)

// Action represents user operation type and other information to
// repository. It implemented interface base.Actioner so that can be
// used in template render.
type Action struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64 `xorm:"INDEX"` // Receiver user id.
	OpType      ActionType
	ActUserID   int64                  `xorm:"INDEX"` // Action user id.
	ActUser     *user_model.User       `xorm:"-"`
	RepoID      int64                  `xorm:"INDEX"`
	Repo        *repo_model.Repository `xorm:"-"`
	CommentID   int64                  `xorm:"INDEX"`
	Comment     *Comment               `xorm:"-"`
	IsDeleted   bool                   `xorm:"INDEX NOT NULL DEFAULT false"`
	RefName     string
	IsPrivate   bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	Content     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

func init() {
	db.RegisterModel(new(Action))
}

// GetOpType gets the ActionType of this action.
func (a *Action) GetOpType() ActionType {
	return a.OpType
}

// LoadActUser loads a.ActUser
func (a *Action) LoadActUser() {
	if a.ActUser != nil {
		return
	}
	var err error
	a.ActUser, err = user_model.GetUserByID(a.ActUserID)
	if err == nil {
		return
	} else if user_model.IsErrUserNotExist(err) {
		a.ActUser = user_model.NewGhostUser()
	} else {
		log.Error("GetUserByID(%d): %v", a.ActUserID, err)
	}
}

func (a *Action) loadRepo() {
	if a.Repo != nil {
		return
	}
	var err error
	a.Repo, err = repo_model.GetRepositoryByID(a.RepoID)
	if err != nil {
		log.Error("repo_model.GetRepositoryByID(%d): %v", a.RepoID, err)
	}
}

// GetActFullName gets the action's user full name.
func (a *Action) GetActFullName() string {
	a.LoadActUser()
	return a.ActUser.FullName
}

// GetActUserName gets the action's user name.
func (a *Action) GetActUserName() string {
	a.LoadActUser()
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
	a.loadRepo()
	return a.Repo.OwnerName
}

// ShortRepoUserName returns the name of the action repository owner
// trimmed to max 20 chars.
func (a *Action) ShortRepoUserName() string {
	return base.EllipsisString(a.GetRepoUserName(), 20)
}

// GetRepoName returns the name of the action repository.
func (a *Action) GetRepoName() string {
	a.loadRepo()
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

// GetRepositoryFromMatch returns a *repo_model.Repository from a username and repo strings
func GetRepositoryFromMatch(ownerName, repoName string) (*repo_model.Repository, error) {
	var err error
	refRepo, err := repo_model.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			log.Warn("Repository referenced in commit but does not exist: %v", err)
			return nil, err
		}
		log.Error("repo_model.GetRepositoryByOwnerAndName: %v", err)
		return nil, err
	}
	return refRepo, nil
}

// GetCommentLink returns link to action comment.
func (a *Action) GetCommentLink() string {
	return a.getCommentLink(db.DefaultContext)
}

func (a *Action) getCommentLink(ctx context.Context) string {
	if a == nil {
		return "#"
	}
	e := db.GetEngine(ctx)
	if a.Comment == nil && a.CommentID != 0 {
		a.Comment, _ = getCommentByID(e, a.CommentID)
	}
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

	issue, err := getIssueByID(e, issueID)
	if err != nil {
		return "#"
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return "#"
	}

	return issue.HTMLURL()
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
	case len(a.RefName) == 40 && git.SHAPattern.MatchString(a.RefName):
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
	issue, err := GetIssueByIndex(a.RepoID, index)
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
	issue, err := GetIssueByIndex(a.RepoID, index)
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
func GetFeeds(ctx context.Context, opts GetFeedsOptions) (ActionList, error) {
	if opts.RequestedUser == nil && opts.RequestedTeam == nil && opts.RequestedRepo == nil {
		return nil, fmt.Errorf("need at least one of these filters: RequestedUser, RequestedTeam, RequestedRepo")
	}

	cond, err := activityQueryCondition(opts)
	if err != nil {
		return nil, err
	}

	e := db.GetEngine(ctx)
	sess := e.Where(cond)

	opts.SetDefaultValues()
	sess = db.SetSessionPagination(sess, &opts)

	actions := make([]*Action, 0, opts.PageSize)

	if err := sess.Desc("created_unix").Find(&actions); err != nil {
		return nil, fmt.Errorf("Find: %v", err)
	}

	if err := ActionList(actions).loadAttributes(e); err != nil {
		return nil, fmt.Errorf("LoadAttributes: %v", err)
	}

	return actions, nil
}

func activityReadable(user, doer *user_model.User) bool {
	return !user.KeepActivityPrivate ||
		doer != nil && (doer.IsAdmin || user.ID == doer.ID)
}

func activityQueryCondition(opts GetFeedsOptions) (builder.Cond, error) {
	cond := builder.NewCond()

	if opts.RequestedTeam != nil && opts.RequestedUser == nil {
		org, err := user_model.GetUserByID(opts.RequestedTeam.OrgID)
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
		cond = cond.And(builder.In("repo_id", AccessibleRepoIDsQuery(opts.Actor)))
	}

	if opts.RequestedRepo != nil {
		cond = cond.And(builder.Eq{"repo_id": opts.RequestedRepo.ID})
	}

	if opts.RequestedTeam != nil {
		env := organization.OrgFromUser(opts.RequestedUser).AccessibleTeamReposEnv(opts.RequestedTeam)
		teamRepoIDs, err := env.RepoIDs(1, opts.RequestedUser.NumRepos)
		if err != nil {
			return nil, fmt.Errorf("GetTeamRepositories: %v", err)
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
		cond = cond.And(builder.Eq{"is_private": false})
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

			cond = cond.And(builder.Gte{"created_unix": dateLow.Unix()})
			cond = cond.And(builder.Lte{"created_unix": dateHigh.Unix()})
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
	return
}

func notifyWatchers(ctx context.Context, actions ...*Action) error {
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
				return fmt.Errorf("get watchers: %v", err)
			}
		}

		// Add feed for actioner.
		act.UserID = act.ActUserID
		if _, err = e.Insert(act); err != nil {
			return fmt.Errorf("insert new actioner: %v", err)
		}

		if repoChanged {
			act.loadRepo()
			repo = act.Repo

			// check repo owner exist.
			if err := act.Repo.GetOwner(ctx); err != nil {
				return fmt.Errorf("can't get repo owner: %v", err)
			}
		} else if act.Repo == nil {
			act.Repo = repo
		}

		// Add feed for organization
		if act.Repo.Owner.IsOrganization() && act.ActUserID != act.Repo.Owner.ID {
			act.ID = 0
			act.UserID = act.Repo.Owner.ID
			if _, err = e.InsertOne(act); err != nil {
				return fmt.Errorf("insert new actioner: %v", err)
			}
		}

		if repoChanged {
			permCode = make([]bool, len(watchers))
			permIssue = make([]bool, len(watchers))
			permPR = make([]bool, len(watchers))
			for i, watcher := range watchers {
				user, err := user_model.GetUserByIDEngine(e, watcher.UserID)
				if err != nil {
					permCode[i] = false
					permIssue[i] = false
					permPR[i] = false
					continue
				}
				perm, err := GetUserRepoPermission(ctx, repo, user)
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
			case ActionCreatePullRequest, ActionCommentPull, ActionMergePullRequest, ActionClosePullRequest, ActionReopenPullRequest:
				if !permPR[i] {
					continue
				}
			}

			if _, err = e.InsertOne(act); err != nil {
				return fmt.Errorf("insert new action: %v", err)
			}
		}
	}
	return nil
}

// NotifyWatchers creates batch of actions for every watcher.
func NotifyWatchers(actions ...*Action) error {
	return notifyWatchers(db.DefaultContext, actions...)
}

// NotifyWatchersActions creates batch of actions for every watcher.
func NotifyWatchersActions(acts []*Action) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	for _, act := range acts {
		if err := notifyWatchers(ctx, act); err != nil {
			return err
		}
	}
	return committer.Commit()
}
