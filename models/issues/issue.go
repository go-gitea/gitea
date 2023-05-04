// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
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

	DeadlineUnix timeutil.TimeStamp `xorm:"INDEX"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedUnix  timeutil.TimeStamp `xorm:"INDEX"`

	Attachments      []*repo_model.Attachment `xorm:"-"`
	Comments         []*Comment               `xorm:"-"`
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

// LoadLabels loads labels
func (issue *Issue) LoadLabels(ctx context.Context) (err error) {
	if issue.Labels == nil && issue.ID != 0 {
		issue.Labels, err = GetLabelsByIssueID(ctx, issue.ID)
		if err != nil {
			return fmt.Errorf("getLabelsByIssueID [%d]: %w", issue.ID, err)
		}
	}
	return nil
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

	if err = CommentList(issue.Comments).loadAttributes(ctx); err != nil {
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

func (issue *Issue) getLabels(ctx context.Context) (err error) {
	if len(issue.Labels) > 0 {
		return nil
	}

	issue.Labels, err = GetLabelsByIssueID(ctx, issue.ID)
	if err != nil {
		return fmt.Errorf("getLabelsByIssueID: %w", err)
	}
	return nil
}

func clearIssueLabels(ctx context.Context, issue *Issue, doer *user_model.User) (err error) {
	if err = issue.getLabels(ctx); err != nil {
		return fmt.Errorf("getLabels: %w", err)
	}

	for i := range issue.Labels {
		if err = deleteIssueLabel(ctx, issue, issue.Labels[i], doer); err != nil {
			return fmt.Errorf("removeLabel: %w", err)
		}
	}

	return nil
}

// ClearIssueLabels removes all issue labels as the given user.
// Triggers appropriate WebHooks, if any.
func ClearIssueLabels(issue *Issue, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	} else if err = issue.LoadPullRequest(ctx); err != nil {
		return err
	}

	perm, err := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWriteIssuesOrPulls(issue.IsPull) {
		return ErrRepoLabelNotExist{}
	}

	if err = clearIssueLabels(ctx, issue, doer); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	return nil
}

type labelSorter []*Label

func (ts labelSorter) Len() int {
	return len([]*Label(ts))
}

func (ts labelSorter) Less(i, j int) bool {
	return []*Label(ts)[i].ID < []*Label(ts)[j].ID
}

func (ts labelSorter) Swap(i, j int) {
	[]*Label(ts)[i], []*Label(ts)[j] = []*Label(ts)[j], []*Label(ts)[i]
}

// Ensure only one label of a given scope exists, with labels at the end of the
// array getting preference over earlier ones.
func RemoveDuplicateExclusiveLabels(labels []*Label) []*Label {
	validLabels := make([]*Label, 0, len(labels))

	for i, label := range labels {
		scope := label.ExclusiveScope()
		if scope != "" {
			foundOther := false
			for _, otherLabel := range labels[i+1:] {
				if otherLabel.ExclusiveScope() == scope {
					foundOther = true
					break
				}
			}
			if foundOther {
				continue
			}
		}
		validLabels = append(validLabels, label)
	}

	return validLabels
}

// ReplaceIssueLabels removes all current labels and add new labels to the issue.
// Triggers appropriate WebHooks, if any.
func ReplaceIssueLabels(issue *Issue, labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = issue.LoadRepo(ctx); err != nil {
		return err
	}

	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	labels = RemoveDuplicateExclusiveLabels(labels)

	sort.Sort(labelSorter(labels))
	sort.Sort(labelSorter(issue.Labels))

	var toAdd, toRemove []*Label

	addIndex, removeIndex := 0, 0
	for addIndex < len(labels) && removeIndex < len(issue.Labels) {
		addLabel := labels[addIndex]
		removeLabel := issue.Labels[removeIndex]
		if addLabel.ID == removeLabel.ID {
			// Silently drop invalid labels
			if removeLabel.RepoID != issue.RepoID && removeLabel.OrgID != issue.Repo.OwnerID {
				toRemove = append(toRemove, removeLabel)
			}

			addIndex++
			removeIndex++
		} else if addLabel.ID < removeLabel.ID {
			// Only add if the label is valid
			if addLabel.RepoID == issue.RepoID || addLabel.OrgID == issue.Repo.OwnerID {
				toAdd = append(toAdd, addLabel)
			}
			addIndex++
		} else {
			toRemove = append(toRemove, removeLabel)
			removeIndex++
		}
	}
	toAdd = append(toAdd, labels[addIndex:]...)
	toRemove = append(toRemove, issue.Labels[removeIndex:]...)

	if len(toAdd) > 0 {
		if err = newIssueLabels(ctx, issue, toAdd, doer); err != nil {
			return fmt.Errorf("addLabels: %w", err)
		}
	}

	for _, l := range toRemove {
		if err = deleteIssueLabel(ctx, issue, l, doer); err != nil {
			return fmt.Errorf("removeLabel: %w", err)
		}
	}

	issue.Labels = nil
	if err = issue.LoadLabels(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

// UpdateIssueCols updates cols of issue
func UpdateIssueCols(ctx context.Context, issue *Issue, cols ...string) error {
	if _, err := db.GetEngine(ctx).ID(issue.ID).Cols(cols...).Update(issue); err != nil {
		return err
	}
	return nil
}

func changeIssueStatus(ctx context.Context, issue *Issue, doer *user_model.User, isClosed, isMergePull bool) (*Comment, error) {
	// Reload the issue
	currentIssue, err := GetIssueByID(ctx, issue.ID)
	if err != nil {
		return nil, err
	}

	// Nothing should be performed if current status is same as target status
	if currentIssue.IsClosed == isClosed {
		if !issue.IsPull {
			return nil, ErrIssueWasClosed{
				ID: issue.ID,
			}
		}
		return nil, ErrPullWasClosed{
			ID: issue.ID,
		}
	}

	issue.IsClosed = isClosed
	return doChangeIssueStatus(ctx, issue, doer, isMergePull)
}

func doChangeIssueStatus(ctx context.Context, issue *Issue, doer *user_model.User, isMergePull bool) (*Comment, error) {
	// Check for open dependencies
	if issue.IsClosed && issue.Repo.IsDependenciesEnabled(ctx) {
		// only check if dependencies are enabled and we're about to close an issue, otherwise reopening an issue would fail when there are unsatisfied dependencies
		noDeps, err := IssueNoDependenciesLeft(ctx, issue)
		if err != nil {
			return nil, err
		}

		if !noDeps {
			return nil, ErrDependenciesLeft{issue.ID}
		}
	}

	if issue.IsClosed {
		issue.ClosedUnix = timeutil.TimeStampNow()
	} else {
		issue.ClosedUnix = 0
	}

	if err := UpdateIssueCols(ctx, issue, "is_closed", "closed_unix"); err != nil {
		return nil, err
	}

	// Update issue count of labels
	if err := issue.getLabels(ctx); err != nil {
		return nil, err
	}
	for idx := range issue.Labels {
		if err := updateLabelCols(ctx, issue.Labels[idx], "num_issues", "num_closed_issue"); err != nil {
			return nil, err
		}
	}

	// Update issue count of milestone
	if issue.MilestoneID > 0 {
		if err := UpdateMilestoneCounters(ctx, issue.MilestoneID); err != nil {
			return nil, err
		}
	}

	// update repository's issue closed number
	if err := repo_model.UpdateRepoIssueNumbers(ctx, issue.RepoID, issue.IsPull, true); err != nil {
		return nil, err
	}

	// New action comment
	cmtType := CommentTypeClose
	if !issue.IsClosed {
		cmtType = CommentTypeReopen
	} else if isMergePull {
		cmtType = CommentTypeMergePull
	}

	return CreateComment(ctx, &CreateCommentOptions{
		Type:  cmtType,
		Doer:  doer,
		Repo:  issue.Repo,
		Issue: issue,
	})
}

// ChangeIssueStatus changes issue status to open or closed.
func ChangeIssueStatus(ctx context.Context, issue *Issue, doer *user_model.User, isClosed bool) (*Comment, error) {
	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}
	if err := issue.LoadPoster(ctx); err != nil {
		return nil, err
	}

	return changeIssueStatus(ctx, issue, doer, isClosed, false)
}

// ChangeIssueTitle changes the title of this issue, as the given user.
func ChangeIssueTitle(ctx context.Context, issue *Issue, doer *user_model.User, oldTitle string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = UpdateIssueCols(ctx, issue, "name"); err != nil {
		return fmt.Errorf("updateIssueCols: %w", err)
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return fmt.Errorf("loadRepo: %w", err)
	}

	opts := &CreateCommentOptions{
		Type:     CommentTypeChangeTitle,
		Doer:     doer,
		Repo:     issue.Repo,
		Issue:    issue,
		OldTitle: oldTitle,
		NewTitle: issue.Title,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return fmt.Errorf("createComment: %w", err)
	}
	if err = issue.AddCrossReferences(ctx, doer, true); err != nil {
		return err
	}

	return committer.Commit()
}

// ChangeIssueRef changes the branch of this issue, as the given user.
func ChangeIssueRef(issue *Issue, doer *user_model.User, oldRef string) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = UpdateIssueCols(ctx, issue, "ref"); err != nil {
		return fmt.Errorf("updateIssueCols: %w", err)
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return fmt.Errorf("loadRepo: %w", err)
	}
	oldRefFriendly := strings.TrimPrefix(oldRef, git.BranchPrefix)
	newRefFriendly := strings.TrimPrefix(issue.Ref, git.BranchPrefix)

	opts := &CreateCommentOptions{
		Type:   CommentTypeChangeIssueRef,
		Doer:   doer,
		Repo:   issue.Repo,
		Issue:  issue,
		OldRef: oldRefFriendly,
		NewRef: newRefFriendly,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return fmt.Errorf("createComment: %w", err)
	}

	return committer.Commit()
}

// AddDeletePRBranchComment adds delete branch comment for pull request issue
func AddDeletePRBranchComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issueID int64, branchName string) error {
	issue, err := GetIssueByID(ctx, issueID)
	if err != nil {
		return err
	}
	opts := &CreateCommentOptions{
		Type:   CommentTypeDeleteBranch,
		Doer:   doer,
		Repo:   repo,
		Issue:  issue,
		OldRef: branchName,
	}
	_, err = CreateComment(ctx, opts)
	return err
}

// UpdateIssueAttachments update attachments by UUIDs for the issue
func UpdateIssueAttachments(issueID int64, uuids []string) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, uuids)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", uuids, err)
	}
	for i := 0; i < len(attachments); i++ {
		attachments[i].IssueID = issueID
		if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
		}
	}
	return committer.Commit()
}

// ChangeIssueContent changes issue content, as the given user.
func ChangeIssueContent(issue *Issue, doer *user_model.User, content string) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	hasContentHistory, err := HasIssueContentHistory(ctx, issue.ID, 0)
	if err != nil {
		return fmt.Errorf("HasIssueContentHistory: %w", err)
	}
	if !hasContentHistory {
		if err = SaveIssueContentHistory(ctx, issue.PosterID, issue.ID, 0,
			issue.CreatedUnix, issue.Content, true); err != nil {
			return fmt.Errorf("SaveIssueContentHistory: %w", err)
		}
	}

	issue.Content = content

	if err = UpdateIssueCols(ctx, issue, "content"); err != nil {
		return fmt.Errorf("UpdateIssueCols: %w", err)
	}

	if err = SaveIssueContentHistory(ctx, doer.ID, issue.ID, 0,
		timeutil.TimeStampNow(), issue.Content, false); err != nil {
		return fmt.Errorf("SaveIssueContentHistory: %w", err)
	}

	if err = issue.AddCrossReferences(ctx, doer, true); err != nil {
		return fmt.Errorf("addCrossReferences: %w", err)
	}

	return committer.Commit()
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

// NewIssueOptions represents the options of a new issue.
type NewIssueOptions struct {
	Repo        *repo_model.Repository
	Issue       *Issue
	LabelIDs    []int64
	Attachments []string // In UUID format.
	IsPull      bool
}

// NewIssueWithIndex creates issue with given index
func NewIssueWithIndex(ctx context.Context, doer *user_model.User, opts NewIssueOptions) (err error) {
	e := db.GetEngine(ctx)
	opts.Issue.Title = strings.TrimSpace(opts.Issue.Title)

	if opts.Issue.MilestoneID > 0 {
		milestone, err := GetMilestoneByRepoID(ctx, opts.Issue.RepoID, opts.Issue.MilestoneID)
		if err != nil && !IsErrMilestoneNotExist(err) {
			return fmt.Errorf("getMilestoneByID: %w", err)
		}

		// Assume milestone is invalid and drop silently.
		opts.Issue.MilestoneID = 0
		if milestone != nil {
			opts.Issue.MilestoneID = milestone.ID
			opts.Issue.Milestone = milestone
		}
	}

	if opts.Issue.Index <= 0 {
		return fmt.Errorf("no issue index provided")
	}
	if opts.Issue.ID > 0 {
		return fmt.Errorf("issue exist")
	}

	if _, err := e.Insert(opts.Issue); err != nil {
		return err
	}

	if opts.Issue.MilestoneID > 0 {
		if err := UpdateMilestoneCounters(ctx, opts.Issue.MilestoneID); err != nil {
			return err
		}

		opts := &CreateCommentOptions{
			Type:           CommentTypeMilestone,
			Doer:           doer,
			Repo:           opts.Repo,
			Issue:          opts.Issue,
			OldMilestoneID: 0,
			MilestoneID:    opts.Issue.MilestoneID,
		}
		if _, err = CreateComment(ctx, opts); err != nil {
			return err
		}
	}

	if err := repo_model.UpdateRepoIssueNumbers(ctx, opts.Issue.RepoID, opts.IsPull, false); err != nil {
		return err
	}

	if len(opts.LabelIDs) > 0 {
		// During the session, SQLite3 driver cannot handle retrieve objects after update something.
		// So we have to get all needed labels first.
		labels := make([]*Label, 0, len(opts.LabelIDs))
		if err = e.In("id", opts.LabelIDs).Find(&labels); err != nil {
			return fmt.Errorf("find all labels [label_ids: %v]: %w", opts.LabelIDs, err)
		}

		if err = opts.Issue.LoadPoster(ctx); err != nil {
			return err
		}

		for _, label := range labels {
			// Silently drop invalid labels.
			if label.RepoID != opts.Repo.ID && label.OrgID != opts.Repo.OwnerID {
				continue
			}

			if err = newIssueLabel(ctx, opts.Issue, label, opts.Issue.Poster); err != nil {
				return fmt.Errorf("addLabel [id: %d]: %w", label.ID, err)
			}
		}
	}

	if err = NewIssueUsers(ctx, opts.Repo, opts.Issue); err != nil {
		return err
	}

	if len(opts.Attachments) > 0 {
		attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, opts.Attachments)
		if err != nil {
			return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", opts.Attachments, err)
		}

		for i := 0; i < len(attachments); i++ {
			attachments[i].IssueID = opts.Issue.ID
			if _, err = e.ID(attachments[i].ID).Update(attachments[i]); err != nil {
				return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
			}
		}
	}
	if err = opts.Issue.LoadAttributes(ctx); err != nil {
		return err
	}

	return opts.Issue.AddCrossReferences(ctx, doer, false)
}

// NewIssue creates new issue with labels for repository.
func NewIssue(repo *repo_model.Repository, issue *Issue, labelIDs []int64, uuids []string) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	idx, err := db.GetNextResourceIndex(ctx, "issue_index", repo.ID)
	if err != nil {
		return fmt.Errorf("generate issue index failed: %w", err)
	}

	issue.Index = idx

	if err = NewIssueWithIndex(ctx, issue.Poster, NewIssueOptions{
		Repo:        repo,
		Issue:       issue,
		LabelIDs:    labelIDs,
		Attachments: uuids,
	}); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) || IsErrNewIssueInsert(err) {
			return err
		}
		return fmt.Errorf("newIssue: %w", err)
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	return nil
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

// IssuesOptions represents options of an issue.
type IssuesOptions struct { //nolint
	db.ListOptions
	RepoID             int64 // overwrites RepoCond if not 0
	RepoCond           builder.Cond
	AssigneeID         int64
	PosterID           int64
	MentionedID        int64
	ReviewRequestedID  int64
	ReviewedID         int64
	SubscriberID       int64
	MilestoneIDs       []int64
	ProjectID          int64
	ProjectBoardID     int64
	IsClosed           util.OptionalBool
	IsPull             util.OptionalBool
	LabelIDs           []int64
	IncludedLabelNames []string
	ExcludedLabelNames []string
	IncludeMilestones  []string
	SortType           string
	IssueIDs           []int64
	UpdatedAfterUnix   int64
	UpdatedBeforeUnix  int64
	// prioritize issues from this repo
	PriorityRepoID int64
	IsArchived     util.OptionalBool
	Org            *organization.Organization // issues permission scope
	Team           *organization.Team         // issues permission scope
	User           *user_model.User           // issues permission scope
}

// sortIssuesSession sort an issues-related session based on the provided
// sortType string
func sortIssuesSession(sess *xorm.Session, sortType string, priorityRepoID int64) {
	switch sortType {
	case "oldest":
		sess.Asc("issue.created_unix").Asc("issue.id")
	case "recentupdate":
		sess.Desc("issue.updated_unix").Desc("issue.created_unix").Desc("issue.id")
	case "leastupdate":
		sess.Asc("issue.updated_unix").Asc("issue.created_unix").Asc("issue.id")
	case "mostcomment":
		sess.Desc("issue.num_comments").Desc("issue.created_unix").Desc("issue.id")
	case "leastcomment":
		sess.Asc("issue.num_comments").Desc("issue.created_unix").Desc("issue.id")
	case "priority":
		sess.Desc("issue.priority").Desc("issue.created_unix").Desc("issue.id")
	case "nearduedate":
		// 253370764800 is 01/01/9999 @ 12:00am (UTC)
		sess.Join("LEFT", "milestone", "issue.milestone_id = milestone.id").
			OrderBy("CASE " +
				"WHEN issue.deadline_unix = 0 AND (milestone.deadline_unix = 0 OR milestone.deadline_unix IS NULL) THEN 253370764800 " +
				"WHEN milestone.deadline_unix = 0 OR milestone.deadline_unix IS NULL THEN issue.deadline_unix " +
				"WHEN milestone.deadline_unix < issue.deadline_unix OR issue.deadline_unix = 0 THEN milestone.deadline_unix " +
				"ELSE issue.deadline_unix END ASC").
			Desc("issue.created_unix").
			Desc("issue.id")
	case "farduedate":
		sess.Join("LEFT", "milestone", "issue.milestone_id = milestone.id").
			OrderBy("CASE " +
				"WHEN milestone.deadline_unix IS NULL THEN issue.deadline_unix " +
				"WHEN milestone.deadline_unix < issue.deadline_unix OR issue.deadline_unix = 0 THEN milestone.deadline_unix " +
				"ELSE issue.deadline_unix END DESC").
			Desc("issue.created_unix").
			Desc("issue.id")
	case "priorityrepo":
		sess.OrderBy("CASE "+
			"WHEN issue.repo_id = ? THEN 1 "+
			"ELSE 2 END ASC", priorityRepoID).
			Desc("issue.created_unix").
			Desc("issue.id")
	case "project-column-sorting":
		sess.Asc("project_issue.sorting").Desc("issue.created_unix").Desc("issue.id")
	default:
		sess.Desc("issue.created_unix").Desc("issue.id")
	}
}

func (opts *IssuesOptions) setupSessionWithLimit(sess *xorm.Session) {
	if opts.Page >= 0 && opts.PageSize > 0 {
		var start int
		if opts.Page == 0 {
			start = 0
		} else {
			start = (opts.Page - 1) * opts.PageSize
		}
		sess.Limit(opts.PageSize, start)
	}
	opts.setupSessionNoLimit(sess)
}

func (opts *IssuesOptions) setupSessionNoLimit(sess *xorm.Session) {
	if len(opts.IssueIDs) > 0 {
		sess.In("issue.id", opts.IssueIDs)
	}

	if opts.RepoID != 0 {
		opts.RepoCond = builder.Eq{"issue.repo_id": opts.RepoID}
	}
	if opts.RepoCond != nil {
		sess.And(opts.RepoCond)
	}

	if !opts.IsClosed.IsNone() {
		sess.And("issue.is_closed=?", opts.IsClosed.IsTrue())
	}

	if opts.AssigneeID > 0 {
		applyAssigneeCondition(sess, opts.AssigneeID)
	}

	if opts.PosterID > 0 {
		applyPosterCondition(sess, opts.PosterID)
	}

	if opts.MentionedID > 0 {
		applyMentionedCondition(sess, opts.MentionedID)
	}

	if opts.ReviewRequestedID > 0 {
		applyReviewRequestedCondition(sess, opts.ReviewRequestedID)
	}

	if opts.ReviewedID > 0 {
		applyReviewedCondition(sess, opts.ReviewedID)
	}

	if opts.SubscriberID > 0 {
		applySubscribedCondition(sess, opts.SubscriberID)
	}

	if len(opts.MilestoneIDs) == 1 && opts.MilestoneIDs[0] == db.NoConditionID {
		sess.And("issue.milestone_id = 0")
	} else if len(opts.MilestoneIDs) > 0 {
		sess.In("issue.milestone_id", opts.MilestoneIDs)
	}

	if opts.UpdatedAfterUnix != 0 {
		sess.And(builder.Gte{"issue.updated_unix": opts.UpdatedAfterUnix})
	}
	if opts.UpdatedBeforeUnix != 0 {
		sess.And(builder.Lte{"issue.updated_unix": opts.UpdatedBeforeUnix})
	}

	if opts.ProjectID > 0 {
		sess.Join("INNER", "project_issue", "issue.id = project_issue.issue_id").
			And("project_issue.project_id=?", opts.ProjectID)
	} else if opts.ProjectID == db.NoConditionID { // show those that are in no project
		sess.And(builder.NotIn("issue.id", builder.Select("issue_id").From("project_issue")))
	}

	if opts.ProjectBoardID != 0 {
		if opts.ProjectBoardID > 0 {
			sess.In("issue.id", builder.Select("issue_id").From("project_issue").Where(builder.Eq{"project_board_id": opts.ProjectBoardID}))
		} else {
			sess.In("issue.id", builder.Select("issue_id").From("project_issue").Where(builder.Eq{"project_board_id": 0}))
		}
	}

	switch opts.IsPull {
	case util.OptionalBoolTrue:
		sess.And("issue.is_pull=?", true)
	case util.OptionalBoolFalse:
		sess.And("issue.is_pull=?", false)
	}

	if opts.IsArchived != util.OptionalBoolNone {
		sess.And(builder.Eq{"repository.is_archived": opts.IsArchived.IsTrue()})
	}

	if opts.LabelIDs != nil {
		for i, labelID := range opts.LabelIDs {
			if labelID > 0 {
				sess.Join("INNER", fmt.Sprintf("issue_label il%d", i),
					fmt.Sprintf("issue.id = il%[1]d.issue_id AND il%[1]d.label_id = %[2]d", i, labelID))
			} else {
				sess.Where("issue.id not in (select issue_id from issue_label where label_id = ?)", -labelID)
			}
		}
	}

	if len(opts.IncludedLabelNames) > 0 {
		sess.In("issue.id", BuildLabelNamesIssueIDsCondition(opts.IncludedLabelNames))
	}

	if len(opts.ExcludedLabelNames) > 0 {
		sess.And(builder.NotIn("issue.id", BuildLabelNamesIssueIDsCondition(opts.ExcludedLabelNames)))
	}

	if len(opts.IncludeMilestones) > 0 {
		sess.In("issue.milestone_id",
			builder.Select("id").
				From("milestone").
				Where(builder.In("name", opts.IncludeMilestones)))
	}

	if opts.User != nil {
		sess.And(issuePullAccessibleRepoCond("issue.repo_id", opts.User.ID, opts.Org, opts.Team, opts.IsPull.IsTrue()))
	}
}

// teamUnitsRepoCond returns query condition for those repo id in the special org team with special units access
func teamUnitsRepoCond(id string, userID, orgID, teamID int64, units ...unit.Type) builder.Cond {
	return builder.In(id,
		builder.Select("repo_id").From("team_repo").Where(
			builder.Eq{
				"team_id": teamID,
			}.And(
				builder.Or(
					// Check if the user is member of the team.
					builder.In(
						"team_id", builder.Select("team_id").From("team_user").Where(
							builder.Eq{
								"uid": userID,
							},
						),
					),
					// Check if the user is in the owner team of the organisation.
					builder.Exists(builder.Select("team_id").From("team_user").
						Where(builder.Eq{
							"org_id": orgID,
							"team_id": builder.Select("id").From("team").Where(
								builder.Eq{
									"org_id":     orgID,
									"lower_name": strings.ToLower(organization.OwnerTeamName),
								}),
							"uid": userID,
						}),
					),
				)).And(
				builder.In(
					"team_id", builder.Select("team_id").From("team_unit").Where(
						builder.Eq{
							"`team_unit`.org_id": orgID,
						}.And(
							builder.In("`team_unit`.type", units),
						),
					),
				),
			),
		))
}

// issuePullAccessibleRepoCond userID must not be zero, this condition require join repository table
func issuePullAccessibleRepoCond(repoIDstr string, userID int64, org *organization.Organization, team *organization.Team, isPull bool) builder.Cond {
	cond := builder.NewCond()
	unitType := unit.TypeIssues
	if isPull {
		unitType = unit.TypePullRequests
	}
	if org != nil {
		if team != nil {
			cond = cond.And(teamUnitsRepoCond(repoIDstr, userID, org.ID, team.ID, unitType)) // special team member repos
		} else {
			cond = cond.And(
				builder.Or(
					repo_model.UserOrgUnitRepoCond(repoIDstr, userID, org.ID, unitType), // team member repos
					repo_model.UserOrgPublicUnitRepoCond(userID, org.ID),                // user org public non-member repos, TODO: check repo has issues
				),
			)
		}
	} else {
		cond = cond.And(
			builder.Or(
				repo_model.UserOwnedRepoCond(userID),                          // owned repos
				repo_model.UserAccessRepoCond(repoIDstr, userID),              // user can access repo in a unit independent way
				repo_model.UserAssignedRepoCond(repoIDstr, userID),            // user has been assigned accessible public repos
				repo_model.UserMentionedRepoCond(repoIDstr, userID),           // user has been mentioned accessible public repos
				repo_model.UserCreateIssueRepoCond(repoIDstr, userID, isPull), // user has created issue/pr accessible public repos
			),
		)
	}
	return cond
}

func applyAssigneeCondition(sess *xorm.Session, assigneeID int64) *xorm.Session {
	return sess.Join("INNER", "issue_assignees", "issue.id = issue_assignees.issue_id").
		And("issue_assignees.assignee_id = ?", assigneeID)
}

func applyPosterCondition(sess *xorm.Session, posterID int64) *xorm.Session {
	return sess.And("issue.poster_id=?", posterID)
}

func applyMentionedCondition(sess *xorm.Session, mentionedID int64) *xorm.Session {
	return sess.Join("INNER", "issue_user", "issue.id = issue_user.issue_id").
		And("issue_user.is_mentioned = ?", true).
		And("issue_user.uid = ?", mentionedID)
}

func applyReviewRequestedCondition(sess *xorm.Session, reviewRequestedID int64) *xorm.Session {
	return sess.Join("INNER", []string{"review", "r"}, "issue.id = r.issue_id").
		And("issue.poster_id <> ?", reviewRequestedID).
		And("r.type = ?", ReviewTypeRequest).
		And("r.reviewer_id = ? and r.id in (select max(id) from review where issue_id = r.issue_id and reviewer_id = r.reviewer_id and type in (?, ?, ?))"+
			" or r.reviewer_team_id in (select team_id from team_user where uid = ?)",
			reviewRequestedID, ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest, reviewRequestedID)
}

func applyReviewedCondition(sess *xorm.Session, reviewedID int64) *xorm.Session {
	// Query for pull requests where you are a reviewer or commenter, excluding
	// any pull requests already returned by the the review requested filter.
	notPoster := builder.Neq{"issue.poster_id": reviewedID}
	reviewed := builder.In("issue.id", builder.
		Select("issue_id").
		From("review").
		Where(builder.And(
			builder.Neq{"type": ReviewTypeRequest},
			builder.Or(
				builder.Eq{"reviewer_id": reviewedID},
				builder.In("reviewer_team_id", builder.
					Select("team_id").
					From("team_user").
					Where(builder.Eq{"uid": reviewedID}),
				),
			),
		)),
	)
	commented := builder.In("issue.id", builder.
		Select("issue_id").
		From("comment").
		Where(builder.And(
			builder.Eq{"poster_id": reviewedID},
			builder.In("type", CommentTypeComment, CommentTypeCode, CommentTypeReview),
		)),
	)
	return sess.And(notPoster, builder.Or(reviewed, commented))
}

func applySubscribedCondition(sess *xorm.Session, subscriberID int64) *xorm.Session {
	return sess.And(
		builder.
			NotIn("issue.id",
				builder.Select("issue_id").
					From("issue_watch").
					Where(builder.Eq{"is_watching": false, "user_id": subscriberID}),
			),
	).And(
		builder.Or(
			builder.In("issue.id", builder.
				Select("issue_id").
				From("issue_watch").
				Where(builder.Eq{"is_watching": true, "user_id": subscriberID}),
			),
			builder.In("issue.id", builder.
				Select("issue_id").
				From("comment").
				Where(builder.Eq{"poster_id": subscriberID}),
			),
			builder.Eq{"issue.poster_id": subscriberID},
			builder.In("issue.repo_id", builder.
				Select("id").
				From("watch").
				Where(builder.And(builder.Eq{"user_id": subscriberID},
					builder.In("mode", repo_model.WatchModeNormal, repo_model.WatchModeAuto))),
			),
		),
	)
}

// CountIssuesByRepo map from repoID to number of issues matching the options
func CountIssuesByRepo(ctx context.Context, opts *IssuesOptions) (map[int64]int64, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")

	opts.setupSessionNoLimit(sess)

	countsSlice := make([]*struct {
		RepoID int64
		Count  int64
	}, 0, 10)
	if err := sess.GroupBy("issue.repo_id").
		Select("issue.repo_id AS repo_id, COUNT(*) AS count").
		Table("issue").
		Find(&countsSlice); err != nil {
		return nil, fmt.Errorf("unable to CountIssuesByRepo: %w", err)
	}

	countMap := make(map[int64]int64, len(countsSlice))
	for _, c := range countsSlice {
		countMap[c.RepoID] = c.Count
	}
	return countMap, nil
}

// GetRepoIDsForIssuesOptions find all repo ids for the given options
func GetRepoIDsForIssuesOptions(opts *IssuesOptions, user *user_model.User) ([]int64, error) {
	repoIDs := make([]int64, 0, 5)
	e := db.GetEngine(db.DefaultContext)

	sess := e.Join("INNER", "repository", "`issue`.repo_id = `repository`.id")

	opts.setupSessionNoLimit(sess)

	accessCond := repo_model.AccessibleRepositoryCondition(user, unit.TypeInvalid)
	if err := sess.Where(accessCond).
		Distinct("issue.repo_id").
		Table("issue").
		Find(&repoIDs); err != nil {
		return nil, fmt.Errorf("unable to GetRepoIDsForIssuesOptions: %w", err)
	}

	return repoIDs, nil
}

// Issues returns a list of issues by given conditions.
func Issues(ctx context.Context, opts *IssuesOptions) ([]*Issue, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")
	opts.setupSessionWithLimit(sess)

	sortIssuesSession(sess, opts.SortType, opts.PriorityRepoID)

	issues := make([]*Issue, 0, opts.ListOptions.PageSize)
	if err := sess.Find(&issues); err != nil {
		return nil, fmt.Errorf("unable to query Issues: %w", err)
	}

	if err := IssueList(issues).LoadAttributes(); err != nil {
		return nil, fmt.Errorf("unable to LoadAttributes for Issues: %w", err)
	}

	return issues, nil
}

// CountIssues number return of issues by given conditions.
func CountIssues(ctx context.Context, opts *IssuesOptions) (int64, error) {
	sess := db.GetEngine(ctx).
		Select("COUNT(issue.id) AS count").
		Table("issue").
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")
	opts.setupSessionNoLimit(sess)

	return sess.Count()
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

// UpdateIssueMentions updates issue-user relations for mentioned users.
func UpdateIssueMentions(ctx context.Context, issueID int64, mentions []*user_model.User) error {
	if len(mentions) == 0 {
		return nil
	}
	ids := make([]int64, len(mentions))
	for i, u := range mentions {
		ids[i] = u.ID
	}
	if err := UpdateIssueUsersByMentions(ctx, issueID, ids); err != nil {
		return fmt.Errorf("UpdateIssueUsersByMentions: %w", err)
	}
	return nil
}

// IssueStats represents issue statistic information.
type IssueStats struct {
	OpenCount, ClosedCount int64
	YourRepositoriesCount  int64
	AssignCount            int64
	CreateCount            int64
	MentionCount           int64
	ReviewRequestedCount   int64
	ReviewedCount          int64
}

// Filter modes.
const (
	FilterModeAll = iota
	FilterModeAssign
	FilterModeCreate
	FilterModeMention
	FilterModeReviewRequested
	FilterModeReviewed
	FilterModeYourRepositories
)

// IssueStatsOptions contains parameters accepted by GetIssueStats.
type IssueStatsOptions struct {
	RepoID            int64
	Labels            string
	MilestoneID       int64
	ProjectID         int64
	AssigneeID        int64
	MentionedID       int64
	PosterID          int64
	ReviewRequestedID int64
	ReviewedID        int64
	IsPull            util.OptionalBool
	IssueIDs          []int64
}

const (
	// MaxQueryParameters represents the max query parameters
	// When queries are broken down in parts because of the number
	// of parameters, attempt to break by this amount
	MaxQueryParameters = 300
)

// GetIssueStats returns issue statistic information by given conditions.
func GetIssueStats(opts *IssueStatsOptions) (*IssueStats, error) {
	if len(opts.IssueIDs) <= MaxQueryParameters {
		return getIssueStatsChunk(opts, opts.IssueIDs)
	}

	// If too long a list of IDs is provided, we get the statistics in
	// smaller chunks and get accumulates. Note: this could potentially
	// get us invalid results. The alternative is to insert the list of
	// ids in a temporary table and join from them.
	accum := &IssueStats{}
	for i := 0; i < len(opts.IssueIDs); {
		chunk := i + MaxQueryParameters
		if chunk > len(opts.IssueIDs) {
			chunk = len(opts.IssueIDs)
		}
		stats, err := getIssueStatsChunk(opts, opts.IssueIDs[i:chunk])
		if err != nil {
			return nil, err
		}
		accum.OpenCount += stats.OpenCount
		accum.ClosedCount += stats.ClosedCount
		accum.YourRepositoriesCount += stats.YourRepositoriesCount
		accum.AssignCount += stats.AssignCount
		accum.CreateCount += stats.CreateCount
		accum.OpenCount += stats.MentionCount
		accum.ReviewRequestedCount += stats.ReviewRequestedCount
		accum.ReviewedCount += stats.ReviewedCount
		i = chunk
	}
	return accum, nil
}

func getIssueStatsChunk(opts *IssueStatsOptions, issueIDs []int64) (*IssueStats, error) {
	stats := &IssueStats{}

	countSession := func(opts *IssueStatsOptions, issueIDs []int64) *xorm.Session {
		sess := db.GetEngine(db.DefaultContext).
			Where("issue.repo_id = ?", opts.RepoID)

		if len(issueIDs) > 0 {
			sess.In("issue.id", issueIDs)
		}

		if len(opts.Labels) > 0 && opts.Labels != "0" {
			labelIDs, err := base.StringsToInt64s(strings.Split(opts.Labels, ","))
			if err != nil {
				log.Warn("Malformed Labels argument: %s", opts.Labels)
			} else {
				for i, labelID := range labelIDs {
					if labelID > 0 {
						sess.Join("INNER", fmt.Sprintf("issue_label il%d", i),
							fmt.Sprintf("issue.id = il%[1]d.issue_id AND il%[1]d.label_id = %[2]d", i, labelID))
					} else {
						sess.Where("issue.id NOT IN (SELECT issue_id FROM issue_label WHERE label_id = ?)", -labelID)
					}
				}
			}
		}

		if opts.MilestoneID > 0 {
			sess.And("issue.milestone_id = ?", opts.MilestoneID)
		} else if opts.MilestoneID == db.NoConditionID {
			sess.And("issue.milestone_id = 0")
		}

		if opts.ProjectID > 0 {
			sess.Join("INNER", "project_issue", "issue.id = project_issue.issue_id").
				And("project_issue.project_id=?", opts.ProjectID)
		}

		if opts.AssigneeID > 0 {
			applyAssigneeCondition(sess, opts.AssigneeID)
		}

		if opts.PosterID > 0 {
			applyPosterCondition(sess, opts.PosterID)
		}

		if opts.MentionedID > 0 {
			applyMentionedCondition(sess, opts.MentionedID)
		}

		if opts.ReviewRequestedID > 0 {
			applyReviewRequestedCondition(sess, opts.ReviewRequestedID)
		}

		if opts.ReviewedID > 0 {
			applyReviewedCondition(sess, opts.ReviewedID)
		}

		switch opts.IsPull {
		case util.OptionalBoolTrue:
			sess.And("issue.is_pull=?", true)
		case util.OptionalBoolFalse:
			sess.And("issue.is_pull=?", false)
		}

		return sess
	}

	var err error
	stats.OpenCount, err = countSession(opts, issueIDs).
		And("issue.is_closed = ?", false).
		Count(new(Issue))
	if err != nil {
		return stats, err
	}
	stats.ClosedCount, err = countSession(opts, issueIDs).
		And("issue.is_closed = ?", true).
		Count(new(Issue))
	return stats, err
}

// UserIssueStatsOptions contains parameters accepted by GetUserIssueStats.
type UserIssueStatsOptions struct {
	UserID     int64
	RepoIDs    []int64
	FilterMode int
	IsPull     bool
	IsClosed   bool
	IssueIDs   []int64
	IsArchived util.OptionalBool
	LabelIDs   []int64
	RepoCond   builder.Cond
	Org        *organization.Organization
	Team       *organization.Team
}

// GetUserIssueStats returns issue statistic information for dashboard by given conditions.
func GetUserIssueStats(opts UserIssueStatsOptions) (*IssueStats, error) {
	var err error
	stats := &IssueStats{}

	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"issue.is_pull": opts.IsPull})
	if len(opts.RepoIDs) > 0 {
		cond = cond.And(builder.In("issue.repo_id", opts.RepoIDs))
	}
	if len(opts.IssueIDs) > 0 {
		cond = cond.And(builder.In("issue.id", opts.IssueIDs))
	}
	if opts.RepoCond != nil {
		cond = cond.And(opts.RepoCond)
	}

	if opts.UserID > 0 {
		cond = cond.And(issuePullAccessibleRepoCond("issue.repo_id", opts.UserID, opts.Org, opts.Team, opts.IsPull))
	}

	sess := func(cond builder.Cond) *xorm.Session {
		s := db.GetEngine(db.DefaultContext).Where(cond)
		if len(opts.LabelIDs) > 0 {
			s.Join("INNER", "issue_label", "issue_label.issue_id = issue.id").
				In("issue_label.label_id", opts.LabelIDs)
		}
		if opts.UserID > 0 || opts.IsArchived != util.OptionalBoolNone {
			s.Join("INNER", "repository", "issue.repo_id = repository.id")
			if opts.IsArchived != util.OptionalBoolNone {
				s.And(builder.Eq{"repository.is_archived": opts.IsArchived.IsTrue()})
			}
		}
		return s
	}

	switch opts.FilterMode {
	case FilterModeAll, FilterModeYourRepositories:
		stats.OpenCount, err = sess(cond).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = sess(cond).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeAssign:
		stats.OpenCount, err = applyAssigneeCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyAssigneeCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeCreate:
		stats.OpenCount, err = applyPosterCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyPosterCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeMention:
		stats.OpenCount, err = applyMentionedCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyMentionedCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeReviewRequested:
		stats.OpenCount, err = applyReviewRequestedCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyReviewRequestedCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	case FilterModeReviewed:
		stats.OpenCount, err = applyReviewedCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", false).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
		stats.ClosedCount, err = applyReviewedCondition(sess(cond), opts.UserID).
			And("issue.is_closed = ?", true).
			Count(new(Issue))
		if err != nil {
			return nil, err
		}
	}

	cond = cond.And(builder.Eq{"issue.is_closed": opts.IsClosed})
	stats.AssignCount, err = applyAssigneeCondition(sess(cond), opts.UserID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.CreateCount, err = applyPosterCondition(sess(cond), opts.UserID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.MentionCount, err = applyMentionedCondition(sess(cond), opts.UserID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.YourRepositoriesCount, err = sess(cond).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.ReviewRequestedCount, err = applyReviewRequestedCondition(sess(cond), opts.UserID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	stats.ReviewedCount, err = applyReviewedCondition(sess(cond), opts.UserID).Count(new(Issue))
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetRepoIssueStats returns number of open and closed repository issues by given filter mode.
func GetRepoIssueStats(repoID, uid int64, filterMode int, isPull bool) (numOpen, numClosed int64) {
	countSession := func(isClosed, isPull bool, repoID int64) *xorm.Session {
		sess := db.GetEngine(db.DefaultContext).
			Where("is_closed = ?", isClosed).
			And("is_pull = ?", isPull).
			And("repo_id = ?", repoID)

		return sess
	}

	openCountSession := countSession(false, isPull, repoID)
	closedCountSession := countSession(true, isPull, repoID)

	switch filterMode {
	case FilterModeAssign:
		applyAssigneeCondition(openCountSession, uid)
		applyAssigneeCondition(closedCountSession, uid)
	case FilterModeCreate:
		applyPosterCondition(openCountSession, uid)
		applyPosterCondition(closedCountSession, uid)
	}

	openResult, _ := openCountSession.Count(new(Issue))
	closedResult, _ := closedCountSession.Count(new(Issue))

	return openResult, closedResult
}

// SearchIssueIDsByKeyword search issues on database
func SearchIssueIDsByKeyword(ctx context.Context, kw string, repoIDs []int64, limit, start int) (int64, []int64, error) {
	repoCond := builder.In("repo_id", repoIDs)
	subQuery := builder.Select("id").From("issue").Where(repoCond)
	cond := builder.And(
		repoCond,
		builder.Or(
			db.BuildCaseInsensitiveLike("name", kw),
			db.BuildCaseInsensitiveLike("content", kw),
			builder.In("id", builder.Select("issue_id").
				From("comment").
				Where(builder.And(
					builder.Eq{"type": CommentTypeComment},
					builder.In("issue_id", subQuery),
					db.BuildCaseInsensitiveLike("content", kw),
				)),
			),
		),
	)

	ids := make([]int64, 0, limit)
	res := make([]struct {
		ID          int64
		UpdatedUnix int64
	}, 0, limit)
	err := db.GetEngine(ctx).Distinct("id", "updated_unix").Table("issue").Where(cond).
		OrderBy("`updated_unix` DESC").Limit(limit, start).
		Find(&res)
	if err != nil {
		return 0, nil, err
	}
	for _, r := range res {
		ids = append(ids, r.ID)
	}

	total, err := db.GetEngine(ctx).Distinct("id").Table("issue").Where(cond).Count()
	if err != nil {
		return 0, nil, err
	}

	return total, ids, nil
}

// UpdateIssueByAPI updates all allowed fields of given issue.
// If the issue status is changed a statusChangeComment is returned
// similarly if the title is changed the titleChanged bool is set to true
func UpdateIssueByAPI(issue *Issue, doer *user_model.User) (statusChangeComment *Comment, titleChanged bool, err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return nil, false, err
	}
	defer committer.Close()

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, false, fmt.Errorf("loadRepo: %w", err)
	}

	// Reload the issue
	currentIssue, err := GetIssueByID(ctx, issue.ID)
	if err != nil {
		return nil, false, err
	}

	if _, err := db.GetEngine(ctx).ID(issue.ID).Cols(
		"name", "content", "milestone_id", "priority",
		"deadline_unix", "updated_unix", "is_locked").
		Update(issue); err != nil {
		return nil, false, err
	}

	titleChanged = currentIssue.Title != issue.Title
	if titleChanged {
		opts := &CreateCommentOptions{
			Type:     CommentTypeChangeTitle,
			Doer:     doer,
			Repo:     issue.Repo,
			Issue:    issue,
			OldTitle: currentIssue.Title,
			NewTitle: issue.Title,
		}
		_, err := CreateComment(ctx, opts)
		if err != nil {
			return nil, false, fmt.Errorf("createComment: %w", err)
		}
	}

	if currentIssue.IsClosed != issue.IsClosed {
		statusChangeComment, err = doChangeIssueStatus(ctx, issue, doer, false)
		if err != nil {
			return nil, false, err
		}
	}

	if err := issue.AddCrossReferences(ctx, doer, true); err != nil {
		return nil, false, err
	}
	return statusChangeComment, titleChanged, committer.Commit()
}

// UpdateIssueDeadline updates an issue deadline and adds comments. Setting a deadline to 0 means deleting it.
func UpdateIssueDeadline(issue *Issue, deadlineUnix timeutil.TimeStamp, doer *user_model.User) (err error) {
	// if the deadline hasn't changed do nothing
	if issue.DeadlineUnix == deadlineUnix {
		return nil
	}
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Update the deadline
	if err = UpdateIssueCols(ctx, &Issue{ID: issue.ID, DeadlineUnix: deadlineUnix}, "deadline_unix"); err != nil {
		return err
	}

	// Make the comment
	if _, err = createDeadlineComment(ctx, doer, issue, deadlineUnix); err != nil {
		return fmt.Errorf("createRemovedDueDateComment: %w", err)
	}

	return committer.Commit()
}

// DeleteInIssue delete records in beans with external key issue_id = ?
func DeleteInIssue(ctx context.Context, issueID int64, beans ...interface{}) error {
	e := db.GetEngine(ctx)
	for _, bean := range beans {
		if _, err := e.In("issue_id", issueID).Delete(bean); err != nil {
			return err
		}
	}
	return nil
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

// FindAndUpdateIssueMentions finds users mentioned in the given content string, and saves them in the database.
func FindAndUpdateIssueMentions(ctx context.Context, issue *Issue, doer *user_model.User, content string) (mentions []*user_model.User, err error) {
	rawMentions := references.FindAllMentionsMarkdown(content)
	mentions, err = ResolveIssueMentionsByVisibility(ctx, issue, doer, rawMentions)
	if err != nil {
		return nil, fmt.Errorf("UpdateIssueMentions [%d]: %w", issue.ID, err)
	}
	if err = UpdateIssueMentions(ctx, issue.ID, mentions); err != nil {
		return nil, fmt.Errorf("UpdateIssueMentions [%d]: %w", issue.ID, err)
	}
	return mentions, err
}

// ResolveIssueMentionsByVisibility returns the users mentioned in an issue, removing those that
// don't have access to reading it. Teams are expanded into their users, but organizations are ignored.
func ResolveIssueMentionsByVisibility(ctx context.Context, issue *Issue, doer *user_model.User, mentions []string) (users []*user_model.User, err error) {
	if len(mentions) == 0 {
		return
	}
	if err = issue.LoadRepo(ctx); err != nil {
		return
	}

	resolved := make(map[string]bool, 10)
	var mentionTeams []string

	if err := issue.Repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	repoOwnerIsOrg := issue.Repo.Owner.IsOrganization()
	if repoOwnerIsOrg {
		mentionTeams = make([]string, 0, 5)
	}

	resolved[doer.LowerName] = true
	for _, name := range mentions {
		name := strings.ToLower(name)
		if _, ok := resolved[name]; ok {
			continue
		}
		if repoOwnerIsOrg && strings.Contains(name, "/") {
			names := strings.Split(name, "/")
			if len(names) < 2 || names[0] != issue.Repo.Owner.LowerName {
				continue
			}
			mentionTeams = append(mentionTeams, names[1])
			resolved[name] = true
		} else {
			resolved[name] = false
		}
	}

	if issue.Repo.Owner.IsOrganization() && len(mentionTeams) > 0 {
		teams := make([]*organization.Team, 0, len(mentionTeams))
		if err := db.GetEngine(ctx).
			Join("INNER", "team_repo", "team_repo.team_id = team.id").
			Where("team_repo.repo_id=?", issue.Repo.ID).
			In("team.lower_name", mentionTeams).
			Find(&teams); err != nil {
			return nil, fmt.Errorf("find mentioned teams: %w", err)
		}
		if len(teams) != 0 {
			checked := make([]int64, 0, len(teams))
			unittype := unit.TypeIssues
			if issue.IsPull {
				unittype = unit.TypePullRequests
			}
			for _, team := range teams {
				if team.AccessMode >= perm.AccessModeAdmin {
					checked = append(checked, team.ID)
					resolved[issue.Repo.Owner.LowerName+"/"+team.LowerName] = true
					continue
				}
				has, err := db.GetEngine(ctx).Get(&organization.TeamUnit{OrgID: issue.Repo.Owner.ID, TeamID: team.ID, Type: unittype})
				if err != nil {
					return nil, fmt.Errorf("get team units (%d): %w", team.ID, err)
				}
				if has {
					checked = append(checked, team.ID)
					resolved[issue.Repo.Owner.LowerName+"/"+team.LowerName] = true
				}
			}
			if len(checked) != 0 {
				teamusers := make([]*user_model.User, 0, 20)
				if err := db.GetEngine(ctx).
					Join("INNER", "team_user", "team_user.uid = `user`.id").
					In("`team_user`.team_id", checked).
					And("`user`.is_active = ?", true).
					And("`user`.prohibit_login = ?", false).
					Find(&teamusers); err != nil {
					return nil, fmt.Errorf("get teams users: %w", err)
				}
				if len(teamusers) > 0 {
					users = make([]*user_model.User, 0, len(teamusers))
					for _, user := range teamusers {
						if already, ok := resolved[user.LowerName]; !ok || !already {
							users = append(users, user)
							resolved[user.LowerName] = true
						}
					}
				}
			}
		}
	}

	// Remove names already in the list to avoid querying the database if pending names remain
	mentionUsers := make([]string, 0, len(resolved))
	for name, already := range resolved {
		if !already {
			mentionUsers = append(mentionUsers, name)
		}
	}
	if len(mentionUsers) == 0 {
		return
	}

	if users == nil {
		users = make([]*user_model.User, 0, len(mentionUsers))
	}

	unchecked := make([]*user_model.User, 0, len(mentionUsers))
	if err := db.GetEngine(ctx).
		Where("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		In("`user`.lower_name", mentionUsers).
		Find(&unchecked); err != nil {
		return nil, fmt.Errorf("find mentioned users: %w", err)
	}
	for _, user := range unchecked {
		if already := resolved[user.LowerName]; already || user.IsOrganization() {
			continue
		}
		// Normal users must have read access to the referencing issue
		perm, err := access_model.GetUserRepoPermission(ctx, issue.Repo, user)
		if err != nil {
			return nil, fmt.Errorf("GetUserRepoPermission [%d]: %w", user.ID, err)
		}
		if !perm.CanReadIssuesOrPulls(issue.IsPull) {
			continue
		}
		users = append(users, user)
	}

	return users, err
}

// UpdateIssuesMigrationsByType updates all migrated repositories' issues from gitServiceType to replace originalAuthorID to posterID
func UpdateIssuesMigrationsByType(gitServiceType api.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Table("issue").
		Where("repo_id IN (SELECT id FROM repository WHERE original_service_type = ?)", gitServiceType).
		And("original_author_id = ?", originalAuthorID).
		Update(map[string]interface{}{
			"poster_id":          posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
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

// UpdateReactionsMigrationsByType updates all migrated repositories' reactions from gitServiceType to replace originalAuthorID to posterID
func UpdateReactionsMigrationsByType(gitServiceType api.GitServiceType, originalAuthorID string, userID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Table("reaction").
		Where("original_author_id = ?", originalAuthorID).
		And(migratedIssueCond(gitServiceType)).
		Update(map[string]interface{}{
			"user_id":            userID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// DeleteIssuesByRepoID deletes issues by repositories id
func DeleteIssuesByRepoID(ctx context.Context, repoID int64) (attachmentPaths []string, err error) {
	deleteCond := builder.Select("id").From("issue").Where(builder.Eq{"issue.repo_id": repoID})

	sess := db.GetEngine(ctx)
	// Delete content histories
	if _, err = sess.In("issue_id", deleteCond).
		Delete(&ContentHistory{}); err != nil {
		return
	}

	// Delete comments and attachments
	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Comment{}); err != nil {
		return
	}

	// Dependencies for issues in this repository
	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueDependency{}); err != nil {
		return
	}

	// Delete dependencies for issues in other repositories
	if _, err = sess.In("dependency_id", deleteCond).
		Delete(&IssueDependency{}); err != nil {
		return
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueUser{}); err != nil {
		return
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Reaction{}); err != nil {
		return
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueWatch{}); err != nil {
		return
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Stopwatch{}); err != nil {
		return
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&TrackedTime{}); err != nil {
		return
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&project_model.ProjectIssue{}); err != nil {
		return
	}

	if _, err = sess.In("dependent_issue_id", deleteCond).
		Delete(&Comment{}); err != nil {
		return
	}

	var attachments []*repo_model.Attachment
	if err = sess.In("issue_id", deleteCond).
		Find(&attachments); err != nil {
		return
	}

	for j := range attachments {
		attachmentPaths = append(attachmentPaths, attachments[j].RelativePath())
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&repo_model.Attachment{}); err != nil {
		return
	}

	if _, err = db.DeleteByBean(ctx, &Issue{RepoID: repoID}); err != nil {
		return
	}

	return attachmentPaths, err
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

// CountOrphanedIssues count issues without a repo
func CountOrphanedIssues(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).
		Table("issue").
		Join("LEFT", "repository", "issue.repo_id=repository.id").
		Where(builder.IsNull{"repository.id"}).
		Select("COUNT(`issue`.`id`)").
		Count()
}

// DeleteOrphanedIssues delete issues without a repo
func DeleteOrphanedIssues(ctx context.Context) error {
	var attachmentPaths []string
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var ids []int64

		if err := db.GetEngine(ctx).Table("issue").Distinct("issue.repo_id").
			Join("LEFT", "repository", "issue.repo_id=repository.id").
			Where(builder.IsNull{"repository.id"}).GroupBy("issue.repo_id").
			Find(&ids); err != nil {
			return err
		}

		for i := range ids {
			paths, err := DeleteIssuesByRepoID(ctx, ids[i])
			if err != nil {
				return err
			}
			attachmentPaths = append(attachmentPaths, paths...)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Remove issue attachment files.
	for i := range attachmentPaths {
		system_model.RemoveAllWithNotice(db.DefaultContext, "Delete issue attachment", attachmentPaths[i])
	}
	return nil
}

// HasOriginalAuthor returns if an issue was migrated and has an original author.
func (issue *Issue) HasOriginalAuthor() bool {
	return issue.OriginalAuthor != "" && issue.OriginalAuthorID != 0
}
