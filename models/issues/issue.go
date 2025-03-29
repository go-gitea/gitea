// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"html/template"
	"regexp"
	"slices"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
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

var ErrIssueAlreadyChanged = util.NewInvalidArgumentErrorf("the issue is already changed")

// Issue represents an issue or pull request of repository.
type Issue struct {
	ID                int64                  `xorm:"pk autoincr"`
	RepoID            int64                  `xorm:"INDEX UNIQUE(repo_index)"`
	Repo              *repo_model.Repository `xorm:"-"`
	Index             int64                  `xorm:"UNIQUE(repo_index)"` // Index in one repository.
	PosterID          int64                  `xorm:"INDEX"`
	Poster            *user_model.User       `xorm:"-"`
	OriginalAuthor    string
	OriginalAuthorID  int64                  `xorm:"index"`
	Title             string                 `xorm:"name"`
	Content           string                 `xorm:"LONGTEXT"`
	RenderedContent   template.HTML          `xorm:"-"`
	ContentVersion    int                    `xorm:"NOT NULL DEFAULT 0"`
	Labels            []*Label               `xorm:"-"`
	isLabelsLoaded    bool                   `xorm:"-"`
	MilestoneID       int64                  `xorm:"INDEX"`
	Milestone         *Milestone             `xorm:"-"`
	isMilestoneLoaded bool                   `xorm:"-"`
	Project           *project_model.Project `xorm:"-"`
	Priority          int
	AssigneeID        int64            `xorm:"-"`
	Assignee          *user_model.User `xorm:"-"`
	isAssigneeLoaded  bool             `xorm:"-"`
	IsClosed          bool             `xorm:"INDEX"`
	IsRead            bool             `xorm:"-"`
	IsPull            bool             `xorm:"INDEX"` // Indicates whether is a pull request or not.
	PullRequest       *PullRequest     `xorm:"-"`
	NumComments       int

	// TODO: RemoveIssueRef: see "repo/issue/branch_selector_field.tmpl"
	Ref string

	PinOrder int `xorm:"-"` // 0 means not loaded, -1 means loaded but not pinned

	DeadlineUnix timeutil.TimeStamp `xorm:"INDEX"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedUnix  timeutil.TimeStamp `xorm:"INDEX"`

	Attachments         []*repo_model.Attachment `xorm:"-"`
	isAttachmentsLoaded bool                     `xorm:"-"`
	Comments            CommentList              `xorm:"-"`
	Reactions           ReactionList             `xorm:"-"`
	TotalTrackedTime    int64                    `xorm:"-"`
	Assignees           []*user_model.User       `xorm:"-"`

	// IsLocked limits commenting abilities to users on an issue
	// with write access
	IsLocked bool `xorm:"NOT NULL DEFAULT false"`

	// For view issue page.
	ShowRole RoleDescriptor `xorm:"-"`

	// Time estimate
	TimeEstimate int64 `xorm:"NOT NULL DEFAULT 0"`
}

var (
	issueTasksPat     = regexp.MustCompile(`(^\s*[-*]\s\[[\sxX]\]\s.)|(\n\s*[-*]\s\[[\sxX]\]\s.)`)
	issueTasksDonePat = regexp.MustCompile(`(^\s*[-*]\s\[[xX]\]\s.)|(\n\s*[-*]\s\[[xX]\]\s.)`)
)

// IssueIndex represents the issue index table
type IssueIndex db.ResourceIndex

func init() {
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

func (issue *Issue) LoadAttachments(ctx context.Context) (err error) {
	if issue.isAttachmentsLoaded || issue.Attachments != nil {
		return nil
	}

	issue.Attachments, err = repo_model.GetAttachmentsByIssueID(ctx, issue.ID)
	if err != nil {
		return fmt.Errorf("getAttachmentsByIssueID [%d]: %w", issue.ID, err)
	}
	issue.isAttachmentsLoaded = true
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

// LoadPoster loads poster
func (issue *Issue) LoadPoster(ctx context.Context) (err error) {
	if issue.Poster == nil && issue.PosterID != 0 {
		issue.Poster, err = user_model.GetPossibleUserByID(ctx, issue.PosterID)
		if err != nil {
			issue.PosterID = user_model.GhostUserID
			issue.Poster = user_model.NewGhostUser()
			if !user_model.IsErrUserNotExist(err) {
				return fmt.Errorf("getUserByID.(poster) [%d]: %w", issue.PosterID, err)
			}
			return nil
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
	for _, comment := range issue.Comments {
		comment.Issue = issue
	}
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
	if !issue.isMilestoneLoaded && (issue.Milestone == nil || issue.Milestone.ID != issue.MilestoneID) && issue.MilestoneID > 0 {
		issue.Milestone, err = GetMilestoneByRepoID(ctx, issue.RepoID, issue.MilestoneID)
		if err != nil && !IsErrMilestoneNotExist(err) {
			return fmt.Errorf("getMilestoneByRepoID [repo_id: %d, milestone_id: %d]: %w", issue.RepoID, issue.MilestoneID, err)
		}
		issue.isMilestoneLoaded = true
	}
	return nil
}

func (issue *Issue) LoadPinOrder(ctx context.Context) error {
	if issue.PinOrder != 0 {
		return nil
	}
	issuePin, err := GetIssuePin(ctx, issue)
	if err != nil && !db.IsErrNotExist(err) {
		return err
	}

	if issuePin != nil {
		issue.PinOrder = issuePin.PinOrder
	} else {
		issue.PinOrder = -1
	}
	return nil
}

// LoadAttributes loads the attribute of this issue.
func (issue *Issue) LoadAttributes(ctx context.Context) (err error) {
	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	if err = issue.LoadPoster(ctx); err != nil {
		return err
	}

	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	if err = issue.LoadMilestone(ctx); err != nil {
		return err
	}

	if err = issue.LoadProject(ctx); err != nil {
		return err
	}

	if err = issue.LoadAssignees(ctx); err != nil {
		return err
	}

	if err = issue.LoadPullRequest(ctx); err != nil && !IsErrPullRequestNotExist(err) {
		// It is possible pull request is not yet created.
		return err
	}

	if err = issue.LoadAttachments(ctx); err != nil {
		return err
	}

	if err = issue.loadComments(ctx); err != nil {
		return err
	}

	if err = issue.LoadPinOrder(ctx); err != nil {
		return err
	}

	if err = issue.Comments.LoadAttributes(ctx); err != nil {
		return err
	}
	if issue.IsTimetrackerEnabled(ctx) {
		if err = issue.LoadTotalTimes(ctx); err != nil {
			return err
		}
	}

	return issue.loadReactions(ctx)
}

// IsPinned returns if a Issue is pinned
func (issue *Issue) IsPinned() bool {
	if issue.PinOrder == 0 {
		setting.PanicInDevOrTesting("issue's pinorder has not been loaded")
	}
	return issue.PinOrder > 0
}

func (issue *Issue) ResetAttributesLoaded() {
	issue.isLabelsLoaded = false
	issue.isMilestoneLoaded = false
	issue.isAttachmentsLoaded = false
	issue.isAssigneeLoaded = false
}

// GetIsRead load the `IsRead` field of the issue
func (issue *Issue) GetIsRead(ctx context.Context, userID int64) error {
	issueUser := &IssueUser{IssueID: issue.ID, UID: userID}
	if has, err := db.GetEngine(ctx).Get(issueUser); err != nil {
		return err
	} else if !has {
		issue.IsRead = false
		return nil
	}
	issue.IsRead = issueUser.IsRead
	return nil
}

// APIURL returns the absolute APIURL to this issue.
func (issue *Issue) APIURL(ctx context.Context) string {
	if issue.Repo == nil {
		err := issue.LoadRepo(ctx)
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
func (issue *Issue) GetLastComment(ctx context.Context) (*Comment, error) {
	var c Comment
	exist, err := db.GetEngine(ctx).Where("type = ?", CommentTypeComment).
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
func GetIssueByIndex(ctx context.Context, repoID, index int64) (*Issue, error) {
	if index < 1 {
		return nil, ErrIssueNotExist{}
	}
	issue := &Issue{
		RepoID: repoID,
		Index:  index,
	}
	has, err := db.GetEngine(ctx).Get(issue)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrIssueNotExist{0, repoID, index}
	}
	return issue, nil
}

func isPullToCond(isPull optional.Option[bool]) builder.Cond {
	if isPull.Has() {
		return builder.Eq{"is_pull": isPull.Value()}
	}
	return builder.NewCond()
}

func FindLatestUpdatedIssues(ctx context.Context, repoID int64, isPull optional.Option[bool], pageSize int) (IssueList, error) {
	issues := make([]*Issue, 0, pageSize)
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).
		And(isPullToCond(isPull)).
		OrderBy("updated_unix DESC").
		Limit(pageSize).
		Find(&issues)
	return issues, err
}

func FindIssuesSuggestionByKeyword(ctx context.Context, repoID int64, keyword string, isPull optional.Option[bool], excludedID int64, pageSize int) (IssueList, error) {
	cond := builder.NewCond()
	if excludedID > 0 {
		cond = cond.And(builder.Neq{"`id`": excludedID})
	}

	// It seems that GitHub searches both title and content (maybe sorting by the search engine's ranking system?)
	// The first PR (https://github.com/go-gitea/gitea/pull/32327) uses "search indexer" to search "name(title) +  content"
	// But it seems that searching "content" (especially LIKE by DB engine) generates worse (unusable) results.
	// So now (https://github.com/go-gitea/gitea/pull/33538) it only searches "name(title)", leave the improvements to the future.
	cond = cond.And(db.BuildCaseInsensitiveLike("`name`", keyword))

	issues := make([]*Issue, 0, pageSize)
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).
		And(isPullToCond(isPull)).
		And(cond).
		OrderBy("updated_unix DESC, `index` DESC").
		Limit(pageSize).
		Find(&issues)
	return issues, err
}

// GetIssueWithAttrsByIndex returns issue by index in a repository.
func GetIssueWithAttrsByIndex(ctx context.Context, repoID, index int64) (*Issue, error) {
	issue, err := GetIssueByIndex(ctx, repoID, index)
	if err != nil {
		return nil, err
	}
	return issue, issue.LoadAttributes(ctx)
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

// GetIssuesByIDs return issues with the given IDs.
// If keepOrder is true, the order of the returned issues will be the same as the given IDs.
func GetIssuesByIDs(ctx context.Context, issueIDs []int64, keepOrder ...bool) (IssueList, error) {
	issues := make([]*Issue, 0, len(issueIDs))
	if len(issueIDs) == 0 {
		return issues, nil
	}

	if err := db.GetEngine(ctx).In("id", issueIDs).Find(&issues); err != nil {
		return nil, err
	}

	if len(keepOrder) > 0 && keepOrder[0] {
		m := make(map[int64]*Issue, len(issues))
		appended := container.Set[int64]{}
		for _, issue := range issues {
			m[issue.ID] = issue
		}
		issues = issues[:0]
		for _, id := range issueIDs {
			if issue, ok := m[id]; ok && !appended.Contains(id) { // make sure the id is existed and not appended
				appended.Add(id)
				issues = append(issues, issue)
			}
		}
	}

	return issues, nil
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
func IsUserParticipantsOfIssue(ctx context.Context, user *user_model.User, issue *Issue) bool {
	userIDs, err := issue.GetParticipantIDsByIssue(ctx)
	if err != nil {
		log.Error(err.Error())
		return false
	}
	return slices.Contains(userIDs, user.ID)
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
	if !slices.Contains(userIDs, issue.PosterID) {
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
	if opts.Page > 0 {
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

// InsertIssues insert issues to database
func InsertIssues(ctx context.Context, issues ...*Issue) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, issue := range issues {
		if err := insertIssue(ctx, issue); err != nil {
			return err
		}
	}
	return committer.Commit()
}

func insertIssue(ctx context.Context, issue *Issue) error {
	sess := db.GetEngine(ctx)
	if _, err := sess.NoAutoTime().Insert(issue); err != nil {
		return err
	}
	issueLabels := make([]IssueLabel, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		issueLabels = append(issueLabels, IssueLabel{
			IssueID: issue.ID,
			LabelID: label.ID,
		})
	}
	if len(issueLabels) > 0 {
		if _, err := sess.Insert(issueLabels); err != nil {
			return err
		}
	}

	for _, reaction := range issue.Reactions {
		reaction.IssueID = issue.ID
	}

	if len(issue.Reactions) > 0 {
		if _, err := sess.Insert(issue.Reactions); err != nil {
			return err
		}
	}

	return nil
}

// ChangeIssueTimeEstimate changes the plan time of this issue, as the given user.
func ChangeIssueTimeEstimate(ctx context.Context, issue *Issue, doer *user_model.User, timeEstimate int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := UpdateIssueCols(ctx, &Issue{ID: issue.ID, TimeEstimate: timeEstimate}, "time_estimate"); err != nil {
			return fmt.Errorf("updateIssueCols: %w", err)
		}

		if err := issue.LoadRepo(ctx); err != nil {
			return fmt.Errorf("loadRepo: %w", err)
		}

		opts := &CreateCommentOptions{
			Type:    CommentTypeChangeTimeEstimate,
			Doer:    doer,
			Repo:    issue.Repo,
			Issue:   issue,
			Content: fmt.Sprintf("%d", timeEstimate),
		}
		if _, err := CreateComment(ctx, opts); err != nil {
			return fmt.Errorf("createComment: %w", err)
		}
		return nil
	})
}
