// Copyright 2024 The Gitea Authors. All rights reserved.
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
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm/schemas"
)

// UserActivity represents a user activity. Don't add any xorm index comment after the fields.
// All the indexes are defined in the TableIndices method
type UserActivity struct {
	ID          int64 `xorm:"pk autoincr"`
	OpType      ActionType
	ActUserID   int64            // Action user id.
	ActUser     *user_model.User `xorm:"-"`
	RepoID      int64
	Repo        *repo_model.Repository `xorm:"-"`
	CommentID   int64
	Comment     *issues_model.Comment `xorm:"-"`
	Issue       *issues_model.Issue   `xorm:"-"` // get the issue id from content
	IsDeleted   bool                  `xorm:"NOT NULL DEFAULT false"`
	RefName     string
	IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
	Content     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(UserActivity))
}

// TableIndices implements xorm's TableIndices interface
func (a *UserActivity) TableIndices() []*schemas.Index {
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
func (a *UserActivity) GetOpType() ActionType {
	return a.OpType
}

// LoadActUser loads a.ActUser
func (a *UserActivity) LoadActUser(ctx context.Context) {
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

func (a *UserActivity) loadRepo(ctx context.Context) {
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
func (a *UserActivity) GetActFullName(ctx context.Context) string {
	a.LoadActUser(ctx)
	return a.ActUser.FullName
}

// GetActUserName gets the action's user name.
func (a *UserActivity) GetActUserName(ctx context.Context) string {
	a.LoadActUser(ctx)
	return a.ActUser.Name
}

// ShortActUserName gets the action's user name trimmed to max 20
// chars.
func (a *UserActivity) ShortActUserName(ctx context.Context) string {
	return base.EllipsisString(a.GetActUserName(ctx), 20)
}

// GetActDisplayName gets the action's display name based on DEFAULT_SHOW_FULL_NAME, or falls back to the username if it is blank.
func (a *UserActivity) GetActDisplayName(ctx context.Context) string {
	if setting.UI.DefaultShowFullName {
		trimmedFullName := strings.TrimSpace(a.GetActFullName(ctx))
		if len(trimmedFullName) > 0 {
			return trimmedFullName
		}
	}
	return a.ShortActUserName(ctx)
}

// GetActDisplayNameTitle gets the action's display name used for the title (tooltip) based on DEFAULT_SHOW_FULL_NAME
func (a *UserActivity) GetActDisplayNameTitle(ctx context.Context) string {
	if setting.UI.DefaultShowFullName {
		return a.ShortActUserName(ctx)
	}
	return a.GetActFullName(ctx)
}

// GetRepoUserName returns the name of the action repository owner.
func (a *UserActivity) GetRepoUserName(ctx context.Context) string {
	a.loadRepo(ctx)
	return a.Repo.OwnerName
}

// ShortRepoUserName returns the name of the action repository owner
// trimmed to max 20 chars.
func (a *UserActivity) ShortRepoUserName(ctx context.Context) string {
	return base.EllipsisString(a.GetRepoUserName(ctx), 20)
}

// GetRepoName returns the name of the action repository.
func (a *UserActivity) GetRepoName(ctx context.Context) string {
	a.loadRepo(ctx)
	return a.Repo.Name
}

// ShortRepoName returns the name of the action repository
// trimmed to max 33 chars.
func (a *UserActivity) ShortRepoName(ctx context.Context) string {
	return base.EllipsisString(a.GetRepoName(ctx), 33)
}

// GetRepoPath returns the virtual path to the action repository.
func (a *UserActivity) GetRepoPath(ctx context.Context) string {
	return path.Join(a.GetRepoUserName(ctx), a.GetRepoName(ctx))
}

// ShortRepoPath returns the virtual path to the action repository
// trimmed to max 20 + 1 + 33 chars.
func (a *UserActivity) ShortRepoPath(ctx context.Context) string {
	return path.Join(a.ShortRepoUserName(ctx), a.ShortRepoName(ctx))
}

// GetRepoLink returns relative link to action repository.
func (a *UserActivity) GetRepoLink(ctx context.Context) string {
	// path.Join will skip empty strings
	return path.Join(setting.AppSubURL, "/", url.PathEscape(a.GetRepoUserName(ctx)), url.PathEscape(a.GetRepoName(ctx)))
}

// GetRepoAbsoluteLink returns the absolute link to action repository.
func (a *UserActivity) GetRepoAbsoluteLink(ctx context.Context) string {
	return setting.AppURL + url.PathEscape(a.GetRepoUserName(ctx)) + "/" + url.PathEscape(a.GetRepoName(ctx))
}

// GetBranch returns the action's repository branch.
func (a *UserActivity) GetBranch() string {
	return strings.TrimPrefix(a.RefName, git.BranchPrefix)
}

// GetRefLink returns the action's ref link.
func (a *UserActivity) GetRefLink(ctx context.Context) string {
	return git.RefURL(a.GetRepoLink(ctx), a.RefName)
}

// GetTag returns the action's repository tag.
func (a *UserActivity) GetTag() string {
	return strings.TrimPrefix(a.RefName, git.TagPrefix)
}

// GetContent returns the action's content.
func (a *UserActivity) GetContent() string {
	return a.Content
}

// GetCreate returns the action creation time.
func (a *UserActivity) GetCreate() time.Time {
	return a.CreatedUnix.AsTime()
}

func (a *UserActivity) IsIssueEvent() bool {
	return a.OpType.InActions("comment_issue", "approve_pull_request", "reject_pull_request", "comment_pull", "merge_pull_request")
}

// GetIssueInfos returns a list of associated information with the action.
func (a *UserActivity) GetIssueInfos() []string {
	// make sure it always returns 3 elements, because there are some access to the a[1] and a[2] without checking the length
	ret := strings.SplitN(a.Content, "|", 3)
	for len(ret) < 3 {
		ret = append(ret, "")
	}
	return ret
}

func (a *UserActivity) getIssueIndex() int64 {
	infos := a.GetIssueInfos()
	if len(infos) == 0 {
		return 0
	}
	index, _ := strconv.ParseInt(infos[0], 10, 64)
	return index
}

func (a *UserActivity) LoadRepo(ctx context.Context) error {
	if a.Repo != nil {
		return nil
	}
	var err error
	a.Repo, err = repo_model.GetRepositoryByID(ctx, a.RepoID)
	return err
}

func (a *UserActivity) loadComment(ctx context.Context) (err error) {
	if a.CommentID == 0 || a.Comment != nil {
		return nil
	}
	a.Comment, err = issues_model.GetCommentByID(ctx, a.CommentID)
	return err
}

// GetCommentHTMLURL returns link to action comment.
func (a *UserActivity) GetCommentHTMLURL(ctx context.Context) string {
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
func (a *UserActivity) GetCommentLink(ctx context.Context) string {
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

func (a *UserActivity) LoadIssue(ctx context.Context) error {
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
func (a *UserActivity) GetIssueTitle(ctx context.Context) string {
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
func (a *UserActivity) GetIssueContent(ctx context.Context) string {
	if err := a.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return "<500 when get issue>"
	}
	if a.Issue == nil {
		return "<Content not found>"
	}
	return a.Issue.Content
}

func NotifyWatchers(ctx context.Context, activity *UserActivity) error {
	watchers, err := repo_model.GetWatchers(ctx, activity.RepoID)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		return notifyWatchers(ctx, activity, watchers)
	})
}

// notifyWatchers creates batch of actions for every watcher.
// It could insert duplicate actions for a repository action, like this:
// * Original action: UserID=1 (the real actor), ActUserID=1
// * Organization action: UserID=100 (the repo's org), ActUserID=1
// * Watcher action: UserID=20 (a user who is watching a repo), ActUserID=1
func notifyWatchers(ctx context.Context, activity *UserActivity, watchers []*repo_model.Watch) error {
	var err error
	var permCode []bool
	var permIssue []bool
	var permPR []bool

	// Add activity
	if err = db.Insert(ctx, activity); err != nil {
		return fmt.Errorf("insert new action: %w", err)
	}

	// Add feed for actioner.
	if err := db.Insert(ctx, &UserFeed{
		UserID:     activity.ActUserID,
		ActivityID: activity.ID,
	}); err != nil {
		return fmt.Errorf("insert new actioner: %w", err)
	}

	if err := activity.LoadRepo(ctx); err != nil {
		return err
	}

	// check repo owner exist.
	if err := activity.Repo.LoadOwner(ctx); err != nil {
		return fmt.Errorf("can't get repo owner: %w", err)
	}

	// Add feed for organization
	if activity.Repo.Owner.IsOrganization() && activity.ActUserID != activity.Repo.Owner.ID {
		if err = db.Insert(ctx, &UserFeed{
			UserID:     activity.Repo.Owner.ID,
			ActivityID: activity.ID,
		}); err != nil {
			return fmt.Errorf("insert new actioner: %w", err)
		}
	}

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
		perm, err := access_model.GetUserRepoPermission(ctx, activity.Repo, user)
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

	for i, watcher := range watchers {
		if activity.ActUserID == watcher.UserID {
			continue
		}

		switch activity.OpType {
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

		if err = db.Insert(ctx, &UserFeed{
			UserID:     watcher.UserID,
			ActivityID: activity.ID,
		}); err != nil {
			return fmt.Errorf("insert new action: %w", err)
		}
	}

	return nil
}

// NotifyWatchersActions creates batch of actions for every watcher.
func NotifyWatchersActions(ctx context.Context, activities []*UserActivity) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	watchersCache := make(map[int64][]*repo_model.Watch, len(activities))
	for _, activity := range activities {
		watchers, ok := watchersCache[activity.RepoID]
		if !ok {
			watchers, err = repo_model.GetWatchers(ctx, activity.RepoID)
			if err != nil {
				return err
			}
			watchersCache[activity.RepoID] = watchers
		}

		if err := notifyWatchers(ctx, activity, watchers); err != nil {
			return err
		}
	}
	return committer.Commit()
}

// FixUserActivityCreatedUnixString set created_unix to zero if it is an empty string
func FixUserActivityCreatedUnixString(ctx context.Context) (int64, error) {
	if setting.Database.Type.IsSQLite3() {
		res, err := db.GetEngine(ctx).Exec(`UPDATE user_activity SET created_unix = 0 WHERE created_unix = ""`)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	return 0, nil
}

// CountActivitiesCreatedUnixString count actions where created_unix is an empty string
func CountActivitiesCreatedUnixString(ctx context.Context) (int64, error) {
	if setting.Database.Type.IsSQLite3() {
		return db.GetEngine(ctx).Where(`created_unix = ""`).Count(new(UserActivity))
	}
	return 0, nil
}
