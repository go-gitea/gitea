// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrIssueNotExist represents a "IssueNotExist" kind of error.
type ErrIssueNotExist struct {
	ID     int64
	RepoID int64
	Index  int64
}

// IsErrIssueNotExist checks if an error is a ErrIssueNotExist.
func IsErrIssueNotExist(err error) bool {
	_, ok := err.(ErrIssueNotExist)
	return ok
}

func (err ErrIssueNotExist) Error() string {
	return fmt.Sprintf("issue does not exist [id: %d, repo_id: %d, index: %d]", err.ID, err.RepoID, err.Index)
}

func (err ErrIssueNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrIssueIsClosed represents a "IssueIsClosed" kind of error.
type ErrIssueIsClosed struct {
	ID     int64
	RepoID int64
	Index  int64
}

// IsErrIssueIsClosed checks if an error is a ErrIssueNotExist.
func IsErrIssueIsClosed(err error) bool {
	_, ok := err.(ErrIssueIsClosed)
	return ok
}

func (err ErrIssueIsClosed) Error() string {
	return fmt.Sprintf("issue is closed [id: %d, repo_id: %d, index: %d]", err.ID, err.RepoID, err.Index)
}

// ErrNewIssueInsert is used when the INSERT statement in newIssue fails
type ErrNewIssueInsert struct {
	OriginalError error
}

// IsErrNewIssueInsert checks if an error is a ErrNewIssueInsert.
func IsErrNewIssueInsert(err error) bool {
	_, ok := err.(ErrNewIssueInsert)
	return ok
}

func (err ErrNewIssueInsert) Error() string {
	return err.OriginalError.Error()
}

// ErrIssueWasClosed is used when close a closed issue
type ErrIssueWasClosed struct {
	ID    int64
	Index int64
}

// IsErrIssueWasClosed checks if an error is a ErrIssueWasClosed.
func IsErrIssueWasClosed(err error) bool {
	_, ok := err.(ErrIssueWasClosed)
	return ok
}

func (err ErrIssueWasClosed) Error() string {
	return fmt.Sprintf("Issue [%d] %d was already closed", err.ID, err.Index)
}

// Issue represents an issue or pull request of repository.
type Issue struct {
	ID               int64                  `xorm:"pk autoincr"`
	RepoID           int64                  `xorm:"INDEX UNIQUE(repo_index)"`
	Repo             *repo_model.Repository `xorm:"-"`
	Index            int64                  `xorm:"UNIQUE(repo_index)"` // Index in one repository.
	PosterID         int64                  `xorm:"INDEX"`
	Poster           *user_model.User       `xorm:"-"`
	OriginalAuthor   string
	OriginalAuthorID int64                  `xorm:"index"`
	Title            string                 `xorm:"name"`
	Content          string                 `xorm:"LONGTEXT"`
	RenderedContent  string                 `xorm:"-"`
	Labels           []*Label               `xorm:"-"`
	MilestoneID      int64                  `xorm:"INDEX"`
	Milestone        *Milestone             `xorm:"-"`
	Project          *project_model.Project `xorm:"-"`
	Priority         int
	AssigneeID       int64            `xorm:"-"`
	Assignee         *user_model.User `xorm:"-"`
	IsClosed         bool             `xorm:"INDEX"`
	IsRead           bool             `xorm:"-"`
	IsPull           bool             `xorm:"INDEX"` // Indicates whether is a pull request or not.
	PullRequest      *PullRequest     `xorm:"-"`
	NumComments      int
	Ref              string
	PinOrder         int `xorm:"DEFAULT 0"`

	DeadlineUnix timeutil.TimeStamp `xorm:"INDEX"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedUnix  timeutil.TimeStamp `xorm:"INDEX"`

	Attachments      []*repo_model.Attachment `xorm:"-"`
	Comments         CommentList              `xorm:"-"`
	Reactions        ReactionList             `xorm:"-"`
	TotalTrackedTime int64                    `xorm:"-"`
	Assignees        []*user_model.User       `xorm:"-"`

	// IsLocked limits commenting abilities to users on an issue
	// with write access
	IsLocked bool `xorm:"NOT NULL DEFAULT false"`

	// For view issue page.
	ShowRole RoleDescriptor `xorm:"-"`
}

var (
	issueTasksPat     *regexp.Regexp
	issueTasksDonePat *regexp.Regexp
)

const (
	issueTasksRegexpStr     = `(^\s*[-*]\s\[[\sxX]\]\s.)|(\n\s*[-*]\s\[[\sxX]\]\s.)`
	issueTasksDoneRegexpStr = `(^\s*[-*]\s\[[xX]\]\s.)|(\n\s*[-*]\s\[[xX]\]\s.)`
)

// IssueIndex represents the issue index table
type IssueIndex db.ResourceIndex

func init() {
	issueTasksPat = regexp.MustCompile(issueTasksRegexpStr)
	issueTasksDonePat = regexp.MustCompile(issueTasksDoneRegexpStr)

	db.RegisterModel(new(Issue))
	db.RegisterModel(new(IssueIndex))
}

// LoadTotalTimes load total tracked time
func (issue *Issue) LoadTotalTimes(ctx context.Context) (err error) {
	opts := FindTrackedTimesOptions{IssueID: issue.ID}
	issue.TotalTrackedTime, err = opts.toSession(db.GetEngine(ctx)).SumInt(&TrackedTime{}, "time")
	if err != nil {
		return err
	}
	return nil
}

// IsOverdue checks if the issue is overdue
func (issue *Issue) IsOverdue() bool {
	if issue.IsClosed {
		return issue.ClosedUnix >= issue.DeadlineUnix
	}
	return timeutil.TimeStampNow() >= issue.DeadlineUnix
}

// LoadRepo loads issue's repository
func (issue *Issue) LoadRepo(ctx context.Context) (err error) {
	if issue.Repo == nil && issue.RepoID != 0 {
		issue.Repo, err = repo_model.GetRepositoryByID(ctx, issue.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %w", issue.RepoID, err)
		}
	}
	return nil
}

// IsTimetrackerEnabled returns true if the repo enables timetracking
func (issue *Issue) IsTimetrackerEnabled(ctx context.Context) bool {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error(fmt.Sprintf("loadRepo: %v", err))
		return false
	}
	return issue.Repo.IsTimetrackerEnabled(ctx)
}

// GetPullRequest returns the issue pull request
func (issue *Issue) GetPullRequest() (pr *PullRequest, err error) {
	if !issue.IsPull {
		return nil, fmt.Errorf("Issue is not a pull request")
	}

	pr, err = GetPullRequestByIssueID(db.DefaultContext, issue.ID)
	if err != nil {
		return nil, err
	}
	pr.Issue = issue
	return pr, err
}

// LoadPoster loads poster
func (issue *Issue) LoadPoster(ctx context.Context) (err error) {
	if issue.Poster == nil && issue.PosterID != 0 {
		issue.Poster, err = user_model.GetPossibleUserByID(ctx, issue.PosterID)
		if err != nil {
			issue.PosterID = -1
			issue.Poster = user_model.NewGhostUser()
			if !user_model.IsErrUserNotExist(err) {
				return fmt.Errorf("getUserByID.(poster) [%d]: %w", issue.PosterID, err)
			}
			err = nil
			return
		}
	}
	return err
}

// LoadPullRequest loads pull request info
func (issue *Issue) LoadPullRequest(ctx context.Context) (err error) {
	if issue.IsPull {
		if issue.PullRequest == nil && issue.ID != 0 {
			issue.PullRequest, err = GetPullRequestByIssueID(ctx, issue.ID)
			if err != nil {
				if IsErrPullRequestNotExist(err) {
					return err
				}
				return fmt.Errorf("getPullRequestByIssueID [%d]: %w", issue.ID, err)
			}
		}
		if issue.PullRequest != nil {
			issue.PullRequest.Issue = issue
		}
	}
	return nil
}

func (issue *Issue) loadComments(ctx context.Context) (err error) {
	return issue.loadCommentsByType(ctx, CommentTypeUndefined)
}

// LoadDiscussComments loads discuss comments
func (issue *Issue) LoadDiscussComments(ctx context.Context) error {
	return issue.loadCommentsByType(ctx, CommentTypeComment)
}

func (issue *Issue) loadCommentsByType(ctx context.Context, tp CommentType) (err error) {
	if issue.Comments != nil {
		return nil
	}
	issue.Comments, err = FindComments(ctx, &FindCommentsOptions{
		IssueID: issue.ID,
		Type:    tp,
	})
	return err
}

func (issue *Issue) loadReactions(ctx context.Context) (err error) {
	if issue.Reactions != nil {
		return nil
	}
	reactions, _, err := FindReactions(ctx, FindReactionsOptions{
		IssueID: issue.ID,
	})
	if err != nil {
		return err
	}
	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}
	// Load reaction user data
	if _, err := reactions.LoadUsers(ctx, issue.Repo); err != nil {
		return err
	}

	// Cache comments to map
	comments := make(map[int64]*Comment)
	for _, comment := range issue.Comments {
		comments[comment.ID] = comment
	}
	// Add reactions either to issue or comment
	for _, react := range reactions {
		if react.CommentID == 0 {
			issue.Reactions = append(issue.Reactions, react)
		} else if comment, ok := comments[react.CommentID]; ok {
			comment.Reactions = append(comment.Reactions, react)
		}
	}
	return nil
}

// LoadMilestone load milestone of this issue.
func (issue *Issue) LoadMilestone(ctx context.Context) (err error) {
	if (issue.Milestone == nil || issue.Milestone.ID != issue.MilestoneID) && issue.MilestoneID > 0 {
		issue.Milestone, err = GetMilestoneByRepoID(ctx, issue.RepoID, issue.MilestoneID)
		if err != nil && !IsErrMilestoneNotExist(err) {
			return fmt.Errorf("getMilestoneByRepoID [repo_id: %d, milestone_id: %d]: %w", issue.RepoID, issue.MilestoneID, err)
		}
	}
	return nil
}

// LoadAttributes loads the attribute of this issue.
func (issue *Issue) LoadAttributes(ctx context.Context) (err error) {
	if err = issue.LoadRepo(ctx); err != nil {
		return
	}

	if err = issue.LoadPoster(ctx); err != nil {
		return
	}

	if err = issue.LoadLabels(ctx); err != nil {
		return
	}

	if err = issue.LoadMilestone(ctx); err != nil {
		return
	}

	if err = issue.LoadProject(ctx); err != nil {
		return
	}

	if err = issue.LoadAssignees(ctx); err != nil {
		return
	}

	if err = issue.LoadPullRequest(ctx); err != nil && !IsErrPullRequestNotExist(err) {
		// It is possible pull request is not yet created.
		return err
	}

	if issue.Attachments == nil {
		issue.Attachments, err = repo_model.GetAttachmentsByIssueID(ctx, issue.ID)
		if err != nil {
			return fmt.Errorf("getAttachmentsByIssueID [%d]: %w", issue.ID, err)
		}
	}

	if err = issue.loadComments(ctx); err != nil {
		return err
	}

	if err = issue.Comments.loadAttributes(ctx); err != nil {
		return err
	}
	if issue.IsTimetrackerEnabled(ctx) {
		if err = issue.LoadTotalTimes(ctx); err != nil {
			return err
		}
	}

	return issue.loadReactions(ctx)
}

// GetIsRead load the `IsRead` field of the issue
func (issue *Issue) GetIsRead(userID int64) error {
	issueUser := &IssueUser{IssueID: issue.ID, UID: userID}
	if has, err := db.GetEngine(db.DefaultContext).Get(issueUser); err != nil {
		return err
	} else if !has {
		issue.IsRead = false
		return nil
	}
	issue.IsRead = issueUser.IsRead
	return nil
}

// APIURL returns the absolute APIURL to this issue.
func (issue *Issue) APIURL() string {
	if issue.Repo == nil {
		err := issue.LoadRepo(db.DefaultContext)
		if err != nil {
			log.Error("Issue[%d].APIURL(): %v", issue.ID, err)
			return ""
		}
	}
	return fmt.Sprintf("%s/issues/%d", issue.Repo.APIURL(), issue.Index)
}

// HTMLURL returns the absolute URL to this issue.
func (issue *Issue) HTMLURL() string {
	var path string
	if issue.IsPull {
		path = "pulls"
	} else {
		path = "issues"
	}
	return fmt.Sprintf("%s/%s/%d", issue.Repo.HTMLURL(), path, issue.Index)
}

// Link returns the issue's relative URL.
func (issue *Issue) Link() string {
	var path string
	if issue.IsPull {
		path = "pulls"
	} else {
		path = "issues"
	}
	return fmt.Sprintf("%s/%s/%d", issue.Repo.Link(), path, issue.Index)
}

// DiffURL returns the absolute URL to this diff
func (issue *Issue) DiffURL() string {
	if issue.IsPull {
		return fmt.Sprintf("%s/pulls/%d.diff", issue.Repo.HTMLURL(), issue.Index)
	}
	return ""
}

// PatchURL returns the absolute URL to this patch
func (issue *Issue) PatchURL() string {
	if issue.IsPull {
		return fmt.Sprintf("%s/pulls/%d.patch", issue.Repo.HTMLURL(), issue.Index)
	}
	return ""
}

// State returns string representation of issue status.
func (issue *Issue) State() api.StateType {
	if issue.IsClosed {
		return api.StateClosed
	}
	return api.StateOpen
}

// HashTag returns unique hash tag for issue.
func (issue *Issue) HashTag() string {
	return fmt.Sprintf("issue-%d", issue.ID)
}

// IsPoster returns true if given user by ID is the poster.
func (issue *Issue) IsPoster(uid int64) bool {
	return issue.OriginalAuthorID == 0 && issue.PosterID == uid
}

// GetTasks returns the amount of tasks in the issues content
func (issue *Issue) GetTasks() int {
	return len(issueTasksPat.FindAllStringIndex(issue.Content, -1))
}

// GetTasksDone returns the amount of completed tasks in the issues content
func (issue *Issue) GetTasksDone() int {
	return len(issueTasksDonePat.FindAllStringIndex(issue.Content, -1))
}

// GetLastEventTimestamp returns the last user visible event timestamp, either the creation of this issue or the close.
func (issue *Issue) GetLastEventTimestamp() timeutil.TimeStamp {
	if issue.IsClosed {
		return issue.ClosedUnix
	}
	return issue.CreatedUnix
}

// GetLastEventLabel returns the localization label for the current issue.
func (issue *Issue) GetLastEventLabel() string {
	if issue.IsClosed {
		if issue.IsPull && issue.PullRequest.HasMerged {
			return "repo.pulls.merged_by"
		}
		return "repo.issues.closed_by"
	}
	return "repo.issues.opened_by"
}

// GetLastComment return last comment for the current issue.
func (issue *Issue) GetLastComment() (*Comment, error) {
	var c Comment
	exist, err := db.GetEngine(db.DefaultContext).Where("type = ?", CommentTypeComment).
		And("issue_id = ?", issue.ID).Desc("created_unix").Get(&c)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, nil
	}
	return &c, nil
}

// GetLastEventLabelFake returns the localization label for the current issue without providing a link in the username.
func (issue *Issue) GetLastEventLabelFake() string {
	if issue.IsClosed {
		if issue.IsPull && issue.PullRequest.HasMerged {
			return "repo.pulls.merged_by_fake"
		}
		return "repo.issues.closed_by_fake"
	}
	return "repo.issues.opened_by_fake"
}

// GetIssueByIndex returns raw issue without loading attributes by index in a repository.
func GetIssueByIndex(repoID, index int64) (*Issue, error) {
	if index < 1 {
		return nil, ErrIssueNotExist{}
	}
	issue := &Issue{
		RepoID: repoID,
		Index:  index,
	}
	has, err := db.GetEngine(db.DefaultContext).Get(issue)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrIssueNotExist{0, repoID, index}
	}
	return issue, nil
}

// GetIssueWithAttrsByIndex returns issue by index in a repository.
func GetIssueWithAttrsByIndex(repoID, index int64) (*Issue, error) {
	issue, err := GetIssueByIndex(repoID, index)
	if err != nil {
		return nil, err
	}
	return issue, issue.LoadAttributes(db.DefaultContext)
}

// GetIssueByID returns an issue by given ID.
func GetIssueByID(ctx context.Context, id int64) (*Issue, error) {
	issue := new(Issue)
	has, err := db.GetEngine(ctx).ID(id).Get(issue)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrIssueNotExist{id, 0, 0}
	}
	return issue, nil
}

// GetIssueWithAttrsByID returns an issue with attributes by given ID.
func GetIssueWithAttrsByID(id int64) (*Issue, error) {
	issue, err := GetIssueByID(db.DefaultContext, id)
	if err != nil {
		return nil, err
	}
	return issue, issue.LoadAttributes(db.DefaultContext)
}

// GetIssuesByIDs return issues with the given IDs.
func GetIssuesByIDs(ctx context.Context, issueIDs []int64) (IssueList, error) {
	issues := make([]*Issue, 0, 10)
	return issues, db.GetEngine(ctx).In("id", issueIDs).Find(&issues)
}

// GetIssueIDsByRepoID returns all issue ids by repo id
func GetIssueIDsByRepoID(ctx context.Context, repoID int64) ([]int64, error) {
	ids := make([]int64, 0, 10)
	err := db.GetEngine(ctx).Table("issue").Cols("id").Where("repo_id = ?", repoID).Find(&ids)
	return ids, err
}

// GetParticipantsIDsByIssueID returns the IDs of all users who participated in comments of an issue,
// but skips joining with `user` for performance reasons.
// User permissions must be verified elsewhere if required.
func GetParticipantsIDsByIssueID(ctx context.Context, issueID int64) ([]int64, error) {
	userIDs := make([]int64, 0, 5)
	return userIDs, db.GetEngine(ctx).
		Table("comment").
		Cols("poster_id").
		Where("issue_id = ?", issueID).
		And("type in (?,?,?)", CommentTypeComment, CommentTypeCode, CommentTypeReview).
		Distinct("poster_id").
		Find(&userIDs)
}

// IsUserParticipantsOfIssue return true if user is participants of an issue
func IsUserParticipantsOfIssue(user *user_model.User, issue *Issue) bool {
	userIDs, err := issue.GetParticipantIDsByIssue(db.DefaultContext)
	if err != nil {
		log.Error(err.Error())
		return false
	}
	return util.SliceContains(userIDs, user.ID)
}

// DependencyInfo represents high level information about an issue which is a dependency of another issue.
type DependencyInfo struct {
	Issue                 `xorm:"extends"`
	repo_model.Repository `xorm:"extends"`
}

// GetParticipantIDsByIssue returns all userIDs who are participated in comments of an issue and issue author
func (issue *Issue) GetParticipantIDsByIssue(ctx context.Context) ([]int64, error) {
	if issue == nil {
		return nil, nil
	}
	userIDs := make([]int64, 0, 5)
	if err := db.GetEngine(ctx).Table("comment").Cols("poster_id").
		Where("`comment`.issue_id = ?", issue.ID).
		And("`comment`.type in (?,?,?)", CommentTypeComment, CommentTypeCode, CommentTypeReview).
		And("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		Join("INNER", "`user`", "`user`.id = `comment`.poster_id").
		Distinct("poster_id").
		Find(&userIDs); err != nil {
		return nil, fmt.Errorf("get poster IDs: %w", err)
	}
	if !util.SliceContains(userIDs, issue.PosterID) {
		return append(userIDs, issue.PosterID), nil
	}
	return userIDs, nil
}

// BlockedByDependencies finds all Dependencies an issue is blocked by
func (issue *Issue) BlockedByDependencies(ctx context.Context, opts db.ListOptions) (issueDeps []*DependencyInfo, err error) {
	sess := db.GetEngine(ctx).
		Table("issue").
		Join("INNER", "repository", "repository.id = issue.repo_id").
		Join("INNER", "issue_dependency", "issue_dependency.dependency_id = issue.id").
		Where("issue_id = ?", issue.ID).
		// sort by repo id then created date, with the issues of the same repo at the beginning of the list
		OrderBy("CASE WHEN issue.repo_id = ? THEN 0 ELSE issue.repo_id END, issue.created_unix DESC", issue.RepoID)
	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, &opts)
	}
	err = sess.Find(&issueDeps)

	for _, depInfo := range issueDeps {
		depInfo.Issue.Repo = &depInfo.Repository
	}

	return issueDeps, err
}

// BlockingDependencies returns all blocking dependencies, aka all other issues a given issue blocks
func (issue *Issue) BlockingDependencies(ctx context.Context) (issueDeps []*DependencyInfo, err error) {
	err = db.GetEngine(ctx).
		Table("issue").
		Join("INNER", "repository", "repository.id = issue.repo_id").
		Join("INNER", "issue_dependency", "issue_dependency.issue_id = issue.id").
		Where("dependency_id = ?", issue.ID).
		// sort by repo id then created date, with the issues of the same repo at the beginning of the list
		OrderBy("CASE WHEN issue.repo_id = ? THEN 0 ELSE issue.repo_id END, issue.created_unix DESC", issue.RepoID).
		Find(&issueDeps)

	for _, depInfo := range issueDeps {
		depInfo.Issue.Repo = &depInfo.Repository
	}

	return issueDeps, err
}

func migratedIssueCond(tp api.GitServiceType) builder.Cond {
	return builder.In("issue_id",
		builder.Select("issue.id").
			From("issue").
			InnerJoin("repository", "issue.repo_id = repository.id").
			Where(builder.Eq{
				"repository.original_service_type": tp,
			}),
	)
}

// RemapExternalUser ExternalUserRemappable interface
func (issue *Issue) RemapExternalUser(externalName string, externalID, userID int64) error {
	issue.OriginalAuthor = externalName
	issue.OriginalAuthorID = externalID
	issue.PosterID = userID
	return nil
}

// GetUserID ExternalUserRemappable interface
func (issue *Issue) GetUserID() int64 { return issue.PosterID }

// GetExternalName ExternalUserRemappable interface
func (issue *Issue) GetExternalName() string { return issue.OriginalAuthor }

// GetExternalID ExternalUserRemappable interface
func (issue *Issue) GetExternalID() int64 { return issue.OriginalAuthorID }

// HasOriginalAuthor returns if an issue was migrated and has an original author.
func (issue *Issue) HasOriginalAuthor() bool {
	return issue.OriginalAuthor != "" && issue.OriginalAuthorID != 0
}

var ErrIssueMaxPinReached = util.NewInvalidArgumentErrorf("the max number of pinned issues has been readched")

// IsPinned returns if a Issue is pinned
func (issue *Issue) IsPinned() bool {
	return issue.PinOrder != 0
}

// Pin pins a Issue
func (issue *Issue) Pin(ctx context.Context, user *user_model.User) error {
	// If the Issue is already pinned, we don't need to pin it twice
	if issue.IsPinned() {
		return nil
	}

	var maxPin int
	_, err := db.GetEngine(ctx).SQL("SELECT MAX(pin_order) FROM issue WHERE repo_id = ? AND is_pull = ?", issue.RepoID, issue.IsPull).Get(&maxPin)
	if err != nil {
		return err
	}

	// Check if the maximum allowed Pins reached
	if maxPin >= setting.Repository.Issue.MaxPinned {
		return ErrIssueMaxPinReached
	}

	_, err = db.GetEngine(ctx).Table("issue").
		Where("id = ?", issue.ID).
		Update(map[string]any{
			"pin_order": maxPin + 1,
		})
	if err != nil {
		return err
	}

	// Add the pin event to the history
	opts := &CreateCommentOptions{
		Type:  CommentTypePin,
		Doer:  user,
		Repo:  issue.Repo,
		Issue: issue,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return err
	}

	return nil
}

// UnpinIssue unpins a Issue
func (issue *Issue) Unpin(ctx context.Context, user *user_model.User) error {
	// If the Issue is not pinned, we don't need to unpin it
	if !issue.IsPinned() {
		return nil
	}

	// This sets the Pin for all Issues that come after the unpined Issue to the correct value
	_, err := db.GetEngine(ctx).Exec("UPDATE issue SET pin_order = pin_order - 1 WHERE repo_id = ? AND is_pull = ? AND pin_order > ?", issue.RepoID, issue.IsPull, issue.PinOrder)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(ctx).Table("issue").
		Where("id = ?", issue.ID).
		Update(map[string]any{
			"pin_order": 0,
		})
	if err != nil {
		return err
	}

	// Add the unpin event to the history
	opts := &CreateCommentOptions{
		Type:  CommentTypeUnpin,
		Doer:  user,
		Repo:  issue.Repo,
		Issue: issue,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return err
	}

	return nil
}

// PinOrUnpin pins or unpins a Issue
func (issue *Issue) PinOrUnpin(ctx context.Context, user *user_model.User) error {
	if !issue.IsPinned() {
		return issue.Pin(ctx, user)
	}

	return issue.Unpin(ctx, user)
}

// MovePin moves a Pinned Issue to a new Position
func (issue *Issue) MovePin(ctx context.Context, newPosition int) error {
	// If the Issue is not pinned, we can't move them
	if !issue.IsPinned() {
		return nil
	}

	if newPosition < 1 {
		return fmt.Errorf("The Position can't be lower than 1")
	}

	dbctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	var maxPin int
	_, err = db.GetEngine(dbctx).SQL("SELECT MAX(pin_order) FROM issue WHERE repo_id = ? AND is_pull = ?", issue.RepoID, issue.IsPull).Get(&maxPin)
	if err != nil {
		return err
	}

	// If the new Position bigger than the current Maximum, set it to the Maximum
	if newPosition > maxPin+1 {
		newPosition = maxPin + 1
	}

	// Lower the Position of all Pinned Issue that came after the current Position
	_, err = db.GetEngine(dbctx).Exec("UPDATE issue SET pin_order = pin_order - 1 WHERE repo_id = ? AND is_pull = ? AND pin_order > ?", issue.RepoID, issue.IsPull, issue.PinOrder)
	if err != nil {
		return err
	}

	// Higher the Position of all Pinned Issues that comes after the new Position
	_, err = db.GetEngine(dbctx).Exec("UPDATE issue SET pin_order = pin_order + 1 WHERE repo_id = ? AND is_pull = ? AND pin_order >= ?", issue.RepoID, issue.IsPull, newPosition)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(dbctx).Table("issue").
		Where("id = ?", issue.ID).
		Update(map[string]any{
			"pin_order": newPosition,
		})
	if err != nil {
		return err
	}

	return committer.Commit()
}

// GetPinnedIssues returns the pinned Issues for the given Repo and type
func GetPinnedIssues(ctx context.Context, repoID int64, isPull bool) ([]*Issue, error) {
	issues := make([]*Issue, 0)

	err := db.GetEngine(ctx).
		Table("issue").
		Where("repo_id = ?", repoID).
		And("is_pull = ?", isPull).
		And("pin_order > 0").
		OrderBy("pin_order").
		Find(&issues)
	if err != nil {
		return nil, err
	}

	err = IssueList(issues).LoadAttributes()
	if err != nil {
		return nil, err
	}

	return issues, nil
}

// IsNewPinnedAllowed returns if a new Issue or Pull request can be pinned
func IsNewPinAllowed(ctx context.Context, repoID int64, isPull bool) (bool, error) {
	var maxPin int
	_, err := db.GetEngine(ctx).SQL("SELECT COUNT(pin_order) FROM issue WHERE repo_id = ? AND is_pull = ? AND pin_order > 0", repoID, isPull).Get(&maxPin)
	if err != nil {
		return false, err
	}

	return maxPin < setting.Repository.Issue.MaxPinned, nil
}

// IsErrIssueMaxPinReached returns if the error is, that the User can't pin more Issues
func IsErrIssueMaxPinReached(err error) bool {
	return err == ErrIssueMaxPinReached
}
