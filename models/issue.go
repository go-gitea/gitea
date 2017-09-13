// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	api "code.gitea.io/sdk/gitea"
	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var (
	errMissingIssueNumber = errors.New("No issue number specified")
	errInvalidIssueNumber = errors.New("Invalid issue number")
)

// Issue represents an issue or pull request of repository.
type Issue struct {
	ID              int64       `xorm:"pk autoincr"`
	RepoID          int64       `xorm:"INDEX UNIQUE(repo_index)"`
	Repo            *Repository `xorm:"-"`
	Index           int64       `xorm:"UNIQUE(repo_index)"` // Index in one repository.
	PosterID        int64       `xorm:"INDEX"`
	Poster          *User       `xorm:"-"`
	Title           string      `xorm:"name"`
	Content         string      `xorm:"TEXT"`
	RenderedContent string      `xorm:"-"`
	Labels          []*Label    `xorm:"-"`
	MilestoneID     int64       `xorm:"INDEX"`
	Milestone       *Milestone  `xorm:"-"`
	Priority        int
	AssigneeID      int64        `xorm:"INDEX"`
	Assignee        *User        `xorm:"-"`
	IsClosed        bool         `xorm:"INDEX"`
	IsRead          bool         `xorm:"-"`
	IsPull          bool         `xorm:"INDEX"` // Indicates whether is a pull request or not.
	PullRequest     *PullRequest `xorm:"-"`
	NumComments     int
	Ref             string

	Deadline     time.Time `xorm:"-"`
	DeadlineUnix int64     `xorm:"INDEX"`
	Created      time.Time `xorm:"-"`
	CreatedUnix  int64     `xorm:"INDEX created"`
	Updated      time.Time `xorm:"-"`
	UpdatedUnix  int64     `xorm:"INDEX updated"`

	Attachments []*Attachment `xorm:"-"`
	Comments    []*Comment    `xorm:"-"`
}

// BeforeUpdate is invoked from XORM before updating this object.
func (issue *Issue) BeforeUpdate() {
	issue.DeadlineUnix = issue.Deadline.Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of
// this object.
func (issue *Issue) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "deadline_unix":
		issue.Deadline = time.Unix(issue.DeadlineUnix, 0).Local()
	case "created_unix":
		issue.Created = time.Unix(issue.CreatedUnix, 0).Local()
	case "updated_unix":
		issue.Updated = time.Unix(issue.UpdatedUnix, 0).Local()
	}
}

func (issue *Issue) loadRepo(e Engine) (err error) {
	if issue.Repo == nil {
		issue.Repo, err = getRepositoryByID(e, issue.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %v", issue.RepoID, err)
		}
	}
	return nil
}

// GetPullRequest returns the issue pull request
func (issue *Issue) GetPullRequest() (pr *PullRequest, err error) {
	if !issue.IsPull {
		return nil, fmt.Errorf("Issue is not a pull request")
	}

	pr, err = getPullRequestByIssueID(x, issue.ID)
	return
}

func (issue *Issue) loadLabels(e Engine) (err error) {
	if issue.Labels == nil {
		issue.Labels, err = getLabelsByIssueID(e, issue.ID)
		if err != nil {
			return fmt.Errorf("getLabelsByIssueID [%d]: %v", issue.ID, err)
		}
	}
	return nil
}

func (issue *Issue) loadPoster(e Engine) (err error) {
	if issue.Poster == nil {
		issue.Poster, err = getUserByID(e, issue.PosterID)
		if err != nil {
			issue.PosterID = -1
			issue.Poster = NewGhostUser()
			if !IsErrUserNotExist(err) {
				return fmt.Errorf("getUserByID.(poster) [%d]: %v", issue.PosterID, err)
			}
			err = nil
			return
		}
	}
	return
}

func (issue *Issue) loadAssignee(e Engine) (err error) {
	if issue.Assignee == nil && issue.AssigneeID > 0 {
		issue.Assignee, err = getUserByID(e, issue.AssigneeID)
		if err != nil {
			issue.AssigneeID = -1
			issue.Assignee = NewGhostUser()
			if !IsErrUserNotExist(err) {
				return fmt.Errorf("getUserByID.(assignee) [%d]: %v", issue.AssigneeID, err)
			}
			err = nil
			return
		}
	}
	return
}

func (issue *Issue) loadPullRequest(e Engine) (err error) {
	if issue.IsPull && issue.PullRequest == nil {
		issue.PullRequest, err = getPullRequestByIssueID(e, issue.ID)
		if err != nil {
			if IsErrPullRequestNotExist(err) {
				return err
			}
			return fmt.Errorf("getPullRequestByIssueID [%d]: %v", issue.ID, err)
		}
	}
	return nil
}

func (issue *Issue) loadAttributes(e Engine) (err error) {
	if err = issue.loadRepo(e); err != nil {
		return
	}

	if err = issue.loadPoster(e); err != nil {
		return
	}

	if err = issue.loadLabels(e); err != nil {
		return
	}

	if issue.Milestone == nil && issue.MilestoneID > 0 {
		issue.Milestone, err = getMilestoneByRepoID(e, issue.RepoID, issue.MilestoneID)
		if err != nil && !IsErrMilestoneNotExist(err) {
			return fmt.Errorf("getMilestoneByRepoID [repo_id: %d, milestone_id: %d]: %v", issue.RepoID, issue.MilestoneID, err)
		}
	}

	if err = issue.loadAssignee(e); err != nil {
		return
	}

	if err = issue.loadPullRequest(e); err != nil && !IsErrPullRequestNotExist(err) {
		// It is possible pull request is not yet created.
		return err
	}

	if issue.Attachments == nil {
		issue.Attachments, err = getAttachmentsByIssueID(e, issue.ID)
		if err != nil {
			return fmt.Errorf("getAttachmentsByIssueID [%d]: %v", issue.ID, err)
		}
	}

	if issue.Comments == nil {
		issue.Comments, err = findComments(e, FindCommentsOptions{
			IssueID: issue.ID,
			Type:    CommentTypeUnknown,
		})
		if err != nil {
			return fmt.Errorf("getCommentsByIssueID [%d]: %v", issue.ID, err)
		}
	}

	return nil
}

// LoadAttributes loads the attribute of this issue.
func (issue *Issue) LoadAttributes() error {
	return issue.loadAttributes(x)
}

// GetIsRead load the `IsRead` field of the issue
func (issue *Issue) GetIsRead(userID int64) error {
	issueUser := &IssueUser{IssueID: issue.ID, UID: userID}
	if has, err := x.Get(issueUser); err != nil {
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
	return issue.Repo.APIURL() + "/" + path.Join("issues", fmt.Sprint(issue.ID))
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

// APIFormat assumes some fields assigned with values:
// Required - Poster, Labels,
// Optional - Milestone, Assignee, PullRequest
func (issue *Issue) APIFormat() *api.Issue {
	apiLabels := make([]*api.Label, len(issue.Labels))
	for i := range issue.Labels {
		apiLabels[i] = issue.Labels[i].APIFormat()
	}

	apiIssue := &api.Issue{
		ID:       issue.ID,
		URL:      issue.APIURL(),
		Index:    issue.Index,
		Poster:   issue.Poster.APIFormat(),
		Title:    issue.Title,
		Body:     issue.Content,
		Labels:   apiLabels,
		State:    issue.State(),
		Comments: issue.NumComments,
		Created:  issue.Created,
		Updated:  issue.Updated,
	}

	if issue.Milestone != nil {
		apiIssue.Milestone = issue.Milestone.APIFormat()
	}
	if issue.Assignee != nil {
		apiIssue.Assignee = issue.Assignee.APIFormat()
	}
	if issue.IsPull {
		apiIssue.PullRequest = &api.PullRequestMeta{
			HasMerged: issue.PullRequest.HasMerged,
		}
		if issue.PullRequest.HasMerged {
			apiIssue.PullRequest.Merged = &issue.PullRequest.Merged
		}
	}

	return apiIssue
}

// HashTag returns unique hash tag for issue.
func (issue *Issue) HashTag() string {
	return "issue-" + com.ToStr(issue.ID)
}

// IsPoster returns true if given user by ID is the poster.
func (issue *Issue) IsPoster(uid int64) bool {
	return issue.PosterID == uid
}

func (issue *Issue) hasLabel(e Engine, labelID int64) bool {
	return hasIssueLabel(e, issue.ID, labelID)
}

// HasLabel returns true if issue has been labeled by given ID.
func (issue *Issue) HasLabel(labelID int64) bool {
	return issue.hasLabel(x, labelID)
}

func (issue *Issue) sendLabelUpdatedWebhook(doer *User) {
	var err error
	if issue.IsPull {
		if err = issue.loadRepo(x); err != nil {
			log.Error(4, "loadRepo: %v", err)
			return
		}
		if err = issue.loadPullRequest(x); err != nil {
			log.Error(4, "loadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(); err != nil {
			log.Error(4, "LoadIssue: %v", err)
			return
		}
		err = PrepareWebhooks(issue.Repo, HookEventPullRequest, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(AccessModeNone),
			Sender:      doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go HookQueue.Add(issue.RepoID)
	}
}

func (issue *Issue) addLabel(e *xorm.Session, label *Label, doer *User) error {
	return newIssueLabel(e, issue, label, doer)
}

// AddLabel adds a new label to the issue.
func (issue *Issue) AddLabel(doer *User, label *Label) error {
	if err := NewIssueLabel(issue, label, doer); err != nil {
		return err
	}

	issue.sendLabelUpdatedWebhook(doer)
	return nil
}

func (issue *Issue) addLabels(e *xorm.Session, labels []*Label, doer *User) error {
	return newIssueLabels(e, issue, labels, doer)
}

// AddLabels adds a list of new labels to the issue.
func (issue *Issue) AddLabels(doer *User, labels []*Label) error {
	if err := NewIssueLabels(issue, labels, doer); err != nil {
		return err
	}

	issue.sendLabelUpdatedWebhook(doer)
	return nil
}

func (issue *Issue) getLabels(e Engine) (err error) {
	if len(issue.Labels) > 0 {
		return nil
	}

	issue.Labels, err = getLabelsByIssueID(e, issue.ID)
	if err != nil {
		return fmt.Errorf("getLabelsByIssueID: %v", err)
	}
	return nil
}

func (issue *Issue) removeLabel(e *xorm.Session, doer *User, label *Label) error {
	return deleteIssueLabel(e, issue, label, doer)
}

// RemoveLabel removes a label from issue by given ID.
func (issue *Issue) RemoveLabel(doer *User, label *Label) error {
	if err := issue.loadRepo(x); err != nil {
		return err
	}

	if has, err := HasAccess(doer.ID, issue.Repo, AccessModeWrite); err != nil {
		return err
	} else if !has {
		return ErrLabelNotExist{}
	}

	if err := DeleteIssueLabel(issue, label, doer); err != nil {
		return err
	}

	issue.sendLabelUpdatedWebhook(doer)
	return nil
}

func (issue *Issue) clearLabels(e *xorm.Session, doer *User) (err error) {
	if err = issue.getLabels(e); err != nil {
		return fmt.Errorf("getLabels: %v", err)
	}

	for i := range issue.Labels {
		if err = issue.removeLabel(e, doer, issue.Labels[i]); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	return nil
}

// ClearLabels removes all issue labels as the given user.
// Triggers appropriate WebHooks, if any.
func (issue *Issue) ClearLabels(doer *User) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err := issue.loadRepo(sess); err != nil {
		return err
	} else if err = issue.loadPullRequest(sess); err != nil {
		return err
	}

	if has, err := hasAccess(sess, doer.ID, issue.Repo, AccessModeWrite); err != nil {
		return err
	} else if !has {
		return ErrLabelNotExist{}
	}

	if err = issue.clearLabels(sess, doer); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	if issue.IsPull {
		err = issue.PullRequest.LoadIssue()
		if err != nil {
			log.Error(4, "LoadIssue: %v", err)
			return
		}
		err = PrepareWebhooks(issue.Repo, HookEventPullRequest, &api.PullRequestPayload{
			Action:      api.HookIssueLabelCleared,
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(AccessModeNone),
			Sender:      doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go HookQueue.Add(issue.RepoID)
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

// ReplaceLabels removes all current labels and add new labels to the issue.
// Triggers appropriate WebHooks, if any.
func (issue *Issue) ReplaceLabels(labels []*Label, doer *User) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = issue.loadLabels(sess); err != nil {
		return err
	}

	sort.Sort(labelSorter(labels))
	sort.Sort(labelSorter(issue.Labels))

	var toAdd, toRemove []*Label

	addIndex, removeIndex := 0, 0
	for addIndex < len(labels) && removeIndex < len(issue.Labels) {
		addLabel := labels[addIndex]
		removeLabel := issue.Labels[removeIndex]
		if addLabel.ID == removeLabel.ID {
			addIndex++
			removeIndex++
		} else if addLabel.ID < removeLabel.ID {
			toAdd = append(toAdd, addLabel)
			addIndex++
		} else {
			toRemove = append(toRemove, removeLabel)
			removeIndex++
		}
	}
	toAdd = append(toAdd, labels[addIndex:]...)
	toRemove = append(toRemove, issue.Labels[removeIndex:]...)

	if len(toAdd) > 0 {
		if err = issue.addLabels(sess, toAdd, doer); err != nil {
			return fmt.Errorf("addLabels: %v", err)
		}
	}

	for _, l := range toRemove {
		if err = issue.removeLabel(sess, doer, l); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	return sess.Commit()
}

// GetAssignee sets the Assignee attribute of this issue.
func (issue *Issue) GetAssignee() (err error) {
	if issue.AssigneeID == 0 || issue.Assignee != nil {
		return nil
	}

	issue.Assignee, err = GetUserByID(issue.AssigneeID)
	if IsErrUserNotExist(err) {
		return nil
	}
	return err
}

// ReadBy sets issue to be read by given user.
func (issue *Issue) ReadBy(userID int64) error {
	if err := UpdateIssueUserByRead(userID, issue.ID); err != nil {
		return err
	}

	if err := setNotificationStatusReadIfUnread(x, userID, issue.ID); err != nil {
		return err
	}

	return nil
}

func updateIssueCols(e Engine, issue *Issue, cols ...string) error {
	if _, err := e.Id(issue.ID).Cols(cols...).Update(issue); err != nil {
		return err
	}
	UpdateIssueIndexer(issue)
	return nil
}

// UpdateIssueCols only updates values of specific columns for given issue.
func UpdateIssueCols(issue *Issue, cols ...string) error {
	return updateIssueCols(x, issue, cols...)
}

func (issue *Issue) changeStatus(e *xorm.Session, doer *User, repo *Repository, isClosed bool) (err error) {
	// Nothing should be performed if current status is same as target status
	if issue.IsClosed == isClosed {
		return nil
	}
	issue.IsClosed = isClosed

	if err = updateIssueCols(e, issue, "is_closed"); err != nil {
		return err
	}

	// Update issue count of labels
	if err = issue.getLabels(e); err != nil {
		return err
	}
	for idx := range issue.Labels {
		if issue.IsClosed {
			issue.Labels[idx].NumClosedIssues++
		} else {
			issue.Labels[idx].NumClosedIssues--
		}
		if err = updateLabel(e, issue.Labels[idx]); err != nil {
			return err
		}
	}

	// Update issue count of milestone
	if err = changeMilestoneIssueStats(e, issue); err != nil {
		return err
	}

	// New action comment
	if _, err = createStatusComment(e, doer, repo, issue); err != nil {
		return err
	}

	return nil
}

// ChangeStatus changes issue status to open or closed.
func (issue *Issue) ChangeStatus(doer *User, repo *Repository, isClosed bool) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = issue.changeStatus(sess, doer, repo, isClosed); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	if issue.IsPull {
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		issue.PullRequest.Issue = issue
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  repo.APIFormat(AccessModeNone),
			Sender:      doer.APIFormat(),
		}
		if isClosed {
			apiPullRequest.Action = api.HookIssueClosed
		} else {
			apiPullRequest.Action = api.HookIssueReOpened
		}
		err = PrepareWebhooks(repo, HookEventPullRequest, apiPullRequest)
	}
	if err != nil {
		log.Error(4, "PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	} else {
		go HookQueue.Add(repo.ID)
	}

	return nil
}

// ChangeTitle changes the title of this issue, as the given user.
func (issue *Issue) ChangeTitle(doer *User, title string) (err error) {
	oldTitle := issue.Title
	issue.Title = title
	sess := x.NewSession()
	defer sess.Close()

	if err = sess.Begin(); err != nil {
		return err
	}

	if err = updateIssueCols(sess, issue, "name"); err != nil {
		return fmt.Errorf("updateIssueCols: %v", err)
	}

	if _, err = createChangeTitleComment(sess, doer, issue.Repo, issue, oldTitle, title); err != nil {
		return fmt.Errorf("createChangeTitleComment: %v", err)
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	if issue.IsPull {
		issue.PullRequest.Issue = issue
		err = PrepareWebhooks(issue.Repo, HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(AccessModeNone),
			Sender:      doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go HookQueue.Add(issue.RepoID)
	}

	return nil
}

// AddDeletePRBranchComment adds delete branch comment for pull request issue
func AddDeletePRBranchComment(doer *User, repo *Repository, issueID int64, branchName string) error {
	issue, err := getIssueByID(x, issueID)
	if err != nil {
		return err
	}
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if _, err := createDeleteBranchComment(sess, doer, repo, issue, branchName); err != nil {
		return err
	}

	return sess.Commit()
}

// ChangeContent changes issue content, as the given user.
func (issue *Issue) ChangeContent(doer *User, content string) (err error) {
	oldContent := issue.Content
	issue.Content = content
	if err = UpdateIssueCols(issue, "content"); err != nil {
		return fmt.Errorf("UpdateIssueCols: %v", err)
	}

	if issue.IsPull {
		issue.PullRequest.Issue = issue
		err = PrepareWebhooks(issue.Repo, HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(AccessModeNone),
			Sender:      doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go HookQueue.Add(issue.RepoID)
	}

	return nil
}

// ChangeAssignee changes the Assignee field of this issue.
func (issue *Issue) ChangeAssignee(doer *User, assigneeID int64) (err error) {
	var oldAssigneeID = issue.AssigneeID
	issue.AssigneeID = assigneeID
	if err = UpdateIssueUserByAssignee(issue); err != nil {
		return fmt.Errorf("UpdateIssueUserByAssignee: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err = issue.loadRepo(sess); err != nil {
		return fmt.Errorf("loadRepo: %v", err)
	}

	if _, err = createAssigneeComment(sess, doer, issue.Repo, issue, oldAssigneeID, assigneeID); err != nil {
		return fmt.Errorf("createAssigneeComment: %v", err)
	}

	issue.Assignee, err = GetUserByID(issue.AssigneeID)
	if err != nil && !IsErrUserNotExist(err) {
		log.Error(4, "GetUserByID [assignee_id: %v]: %v", issue.AssigneeID, err)
		return nil
	}

	// Error not nil here means user does not exist, which is remove assignee.
	isRemoveAssignee := err != nil
	if issue.IsPull {
		issue.PullRequest.Issue = issue
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(AccessModeNone),
			Sender:      doer.APIFormat(),
		}
		if isRemoveAssignee {
			apiPullRequest.Action = api.HookIssueUnassigned
		} else {
			apiPullRequest.Action = api.HookIssueAssigned
		}
		if err := PrepareWebhooks(issue.Repo, HookEventPullRequest, apiPullRequest); err != nil {
			log.Error(4, "PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, isRemoveAssignee, err)
			return nil
		}
	}
	go HookQueue.Add(issue.RepoID)
	return nil
}

// NewIssueOptions represents the options of a new issue.
type NewIssueOptions struct {
	Repo        *Repository
	Issue       *Issue
	LabelIDs    []int64
	Attachments []string // In UUID format.
	IsPull      bool
}

func newIssue(e *xorm.Session, doer *User, opts NewIssueOptions) (err error) {
	opts.Issue.Title = strings.TrimSpace(opts.Issue.Title)
	opts.Issue.Index = opts.Repo.NextIssueIndex()

	if opts.Issue.MilestoneID > 0 {
		milestone, err := getMilestoneByRepoID(e, opts.Issue.RepoID, opts.Issue.MilestoneID)
		if err != nil && !IsErrMilestoneNotExist(err) {
			return fmt.Errorf("getMilestoneByID: %v", err)
		}

		// Assume milestone is invalid and drop silently.
		opts.Issue.MilestoneID = 0
		if milestone != nil {
			opts.Issue.MilestoneID = milestone.ID
			opts.Issue.Milestone = milestone
		}
	}

	if assigneeID := opts.Issue.AssigneeID; assigneeID > 0 {
		valid, err := hasAccess(e, assigneeID, opts.Repo, AccessModeWrite)
		if err != nil {
			return fmt.Errorf("hasAccess [user_id: %d, repo_id: %d]: %v", assigneeID, opts.Repo.ID, err)
		}
		if !valid {
			opts.Issue.AssigneeID = 0
			opts.Issue.Assignee = nil
		}
	}

	// Milestone and assignee validation should happen before insert actual object.
	if _, err = e.Insert(opts.Issue); err != nil {
		return err
	}

	if opts.Issue.MilestoneID > 0 {
		if err = changeMilestoneAssign(e, doer, opts.Issue, -1); err != nil {
			return err
		}
	}

	if opts.Issue.AssigneeID > 0 {
		if err = opts.Issue.loadRepo(e); err != nil {
			return err
		}
		if _, err = createAssigneeComment(e, doer, opts.Issue.Repo, opts.Issue, -1, opts.Issue.AssigneeID); err != nil {
			return err
		}
	}

	if opts.IsPull {
		_, err = e.Exec("UPDATE `repository` SET num_pulls = num_pulls + 1 WHERE id = ?", opts.Issue.RepoID)
	} else {
		_, err = e.Exec("UPDATE `repository` SET num_issues = num_issues + 1 WHERE id = ?", opts.Issue.RepoID)
	}
	if err != nil {
		return err
	}

	if len(opts.LabelIDs) > 0 {
		// During the session, SQLite3 driver cannot handle retrieve objects after update something.
		// So we have to get all needed labels first.
		labels := make([]*Label, 0, len(opts.LabelIDs))
		if err = e.In("id", opts.LabelIDs).Find(&labels); err != nil {
			return fmt.Errorf("find all labels [label_ids: %v]: %v", opts.LabelIDs, err)
		}

		if err = opts.Issue.loadPoster(e); err != nil {
			return err
		}

		for _, label := range labels {
			// Silently drop invalid labels.
			if label.RepoID != opts.Repo.ID {
				continue
			}

			if err = opts.Issue.addLabel(e, label, opts.Issue.Poster); err != nil {
				return fmt.Errorf("addLabel [id: %d]: %v", label.ID, err)
			}
		}
	}

	if err = newIssueUsers(e, opts.Repo, opts.Issue); err != nil {
		return err
	}

	UpdateIssueIndexer(opts.Issue)

	if len(opts.Attachments) > 0 {
		attachments, err := getAttachmentsByUUIDs(e, opts.Attachments)
		if err != nil {
			return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %v", opts.Attachments, err)
		}

		for i := 0; i < len(attachments); i++ {
			attachments[i].IssueID = opts.Issue.ID
			if _, err = e.Id(attachments[i].ID).Update(attachments[i]); err != nil {
				return fmt.Errorf("update attachment [id: %d]: %v", attachments[i].ID, err)
			}
		}
	}

	return opts.Issue.loadAttributes(e)
}

// NewIssue creates new issue with labels for repository.
func NewIssue(repo *Repository, issue *Issue, labelIDs []int64, uuids []string) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = newIssue(sess, issue.Poster, NewIssueOptions{
		Repo:        repo,
		Issue:       issue,
		LabelIDs:    labelIDs,
		Attachments: uuids,
	}); err != nil {
		return fmt.Errorf("newIssue: %v", err)
	}

	if err = sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	if err = NotifyWatchers(&Action{
		ActUserID: issue.Poster.ID,
		ActUser:   issue.Poster,
		OpType:    ActionCreateIssue,
		Content:   fmt.Sprintf("%d|%s", issue.Index, issue.Title),
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
	if err = issue.MailParticipants(); err != nil {
		log.Error(4, "MailParticipants: %v", err)
	}

	return nil
}

// GetIssueByRef returns an Issue specified by a GFM reference.
// See https://help.github.com/articles/writing-on-github#references for more information on the syntax.
func GetIssueByRef(ref string) (*Issue, error) {
	n := strings.IndexByte(ref, byte('#'))
	if n == -1 {
		return nil, errMissingIssueNumber
	}

	index, err := com.StrTo(ref[n+1:]).Int64()
	if err != nil {
		return nil, errInvalidIssueNumber
	}

	repo, err := GetRepositoryByRef(ref[:n])
	if err != nil {
		return nil, err
	}

	issue, err := GetIssueByIndex(repo.ID, index)
	if err != nil {
		return nil, err
	}

	return issue, issue.LoadAttributes()
}

// GetRawIssueByIndex returns raw issue without loading attributes by index in a repository.
func GetRawIssueByIndex(repoID, index int64) (*Issue, error) {
	issue := &Issue{
		RepoID: repoID,
		Index:  index,
	}
	has, err := x.Get(issue)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrIssueNotExist{0, repoID, index}
	}
	return issue, nil
}

// GetIssueByIndex returns issue by index in a repository.
func GetIssueByIndex(repoID, index int64) (*Issue, error) {
	issue, err := GetRawIssueByIndex(repoID, index)
	if err != nil {
		return nil, err
	}
	return issue, issue.LoadAttributes()
}

func getIssueByID(e Engine, id int64) (*Issue, error) {
	issue := new(Issue)
	has, err := e.Id(id).Get(issue)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrIssueNotExist{id, 0, 0}
	}
	return issue, issue.loadAttributes(e)
}

// GetIssueByID returns an issue by given ID.
func GetIssueByID(id int64) (*Issue, error) {
	return getIssueByID(x, id)
}

func getIssuesByIDs(e Engine, issueIDs []int64) ([]*Issue, error) {
	issues := make([]*Issue, 0, 10)
	return issues, e.In("id", issueIDs).Find(&issues)
}

// GetIssuesByIDs return issues with the given IDs.
func GetIssuesByIDs(issueIDs []int64) ([]*Issue, error) {
	return getIssuesByIDs(x, issueIDs)
}

// IssuesOptions represents options of an issue.
type IssuesOptions struct {
	RepoID      int64
	AssigneeID  int64
	PosterID    int64
	MentionedID int64
	MilestoneID int64
	RepoIDs     []int64
	Page        int
	PageSize    int
	IsClosed    util.OptionalBool
	IsPull      util.OptionalBool
	Labels      string
	SortType    string
	IssueIDs    []int64
}

// sortIssuesSession sort an issues-related session based on the provided
// sortType string
func sortIssuesSession(sess *xorm.Session, sortType string) {
	switch sortType {
	case "oldest":
		sess.Asc("issue.created_unix")
	case "recentupdate":
		sess.Desc("issue.updated_unix")
	case "leastupdate":
		sess.Asc("issue.updated_unix")
	case "mostcomment":
		sess.Desc("issue.num_comments")
	case "leastcomment":
		sess.Asc("issue.num_comments")
	case "priority":
		sess.Desc("issue.priority")
	default:
		sess.Desc("issue.created_unix")
	}
}

func (opts *IssuesOptions) setupSession(sess *xorm.Session) error {
	if opts.Page >= 0 && opts.PageSize > 0 {
		var start int
		if opts.Page == 0 {
			start = 0
		} else {
			start = (opts.Page - 1) * opts.PageSize
		}
		sess.Limit(opts.PageSize, start)
	}

	if len(opts.IssueIDs) > 0 {
		sess.In("issue.id", opts.IssueIDs)
	}

	if opts.RepoID > 0 {
		sess.And("issue.repo_id=?", opts.RepoID)
	} else if len(opts.RepoIDs) > 0 {
		// In case repository IDs are provided but actually no repository has issue.
		sess.In("issue.repo_id", opts.RepoIDs)
	}

	switch opts.IsClosed {
	case util.OptionalBoolTrue:
		sess.And("issue.is_closed=?", true)
	case util.OptionalBoolFalse:
		sess.And("issue.is_closed=?", false)
	}

	if opts.AssigneeID > 0 {
		sess.And("issue.assignee_id=?", opts.AssigneeID)
	}

	if opts.PosterID > 0 {
		sess.And("issue.poster_id=?", opts.PosterID)
	}

	if opts.MentionedID > 0 {
		sess.Join("INNER", "issue_user", "issue.id = issue_user.issue_id").
			And("issue_user.is_mentioned = ?", true).
			And("issue_user.uid = ?", opts.MentionedID)
	}

	if opts.MilestoneID > 0 {
		sess.And("issue.milestone_id=?", opts.MilestoneID)
	}

	switch opts.IsPull {
	case util.OptionalBoolTrue:
		sess.And("issue.is_pull=?", true)
	case util.OptionalBoolFalse:
		sess.And("issue.is_pull=?", false)
	}

	if len(opts.Labels) > 0 && opts.Labels != "0" {
		labelIDs, err := base.StringsToInt64s(strings.Split(opts.Labels, ","))
		if err != nil {
			return err
		}
		if len(labelIDs) > 0 {
			sess.
				Join("INNER", "issue_label", "issue.id = issue_label.issue_id").
				In("issue_label.label_id", labelIDs)
		}
	}
	return nil
}

// CountIssuesByRepo map from repoID to number of issues matching the options
func CountIssuesByRepo(opts *IssuesOptions) (map[int64]int64, error) {
	sess := x.NewSession()
	defer sess.Close()

	if err := opts.setupSession(sess); err != nil {
		return nil, err
	}

	countsSlice := make([]*struct {
		RepoID int64
		Count  int64
	}, 0, 10)
	if err := sess.GroupBy("issue.repo_id").
		Select("issue.repo_id AS repo_id, COUNT(*) AS count").
		Table("issue").
		Find(&countsSlice); err != nil {
		return nil, err
	}

	countMap := make(map[int64]int64, len(countsSlice))
	for _, c := range countsSlice {
		countMap[c.RepoID] = c.Count
	}
	return countMap, nil
}

// Issues returns a list of issues by given conditions.
func Issues(opts *IssuesOptions) ([]*Issue, error) {
	sess := x.NewSession()
	defer sess.Close()

	if err := opts.setupSession(sess); err != nil {
		return nil, err
	}
	sortIssuesSession(sess, opts.SortType)

	issues := make([]*Issue, 0, setting.UI.IssuePagingNum)
	if err := sess.Find(&issues); err != nil {
		return nil, fmt.Errorf("Find: %v", err)
	}

	if err := IssueList(issues).LoadAttributes(); err != nil {
		return nil, fmt.Errorf("LoadAttributes: %v", err)
	}

	return issues, nil
}

// GetParticipantsByIssueID returns all users who are participated in comments of an issue.
func GetParticipantsByIssueID(issueID int64) ([]*User, error) {
	return getParticipantsByIssueID(x, issueID)
}

func getParticipantsByIssueID(e Engine, issueID int64) ([]*User, error) {
	userIDs := make([]int64, 0, 5)
	if err := e.Table("comment").Cols("poster_id").
		Where("issue_id = ?", issueID).
		And("type = ?", CommentTypeComment).
		Distinct("poster_id").
		Find(&userIDs); err != nil {
		return nil, fmt.Errorf("get poster IDs: %v", err)
	}
	if len(userIDs) == 0 {
		return nil, nil
	}

	users := make([]*User, 0, len(userIDs))
	return users, e.In("id", userIDs).Find(&users)
}

// UpdateIssueMentions extracts mentioned people from content and
// updates issue-user relations for them.
func UpdateIssueMentions(e Engine, issueID int64, mentions []string) error {
	if len(mentions) == 0 {
		return nil
	}

	for i := range mentions {
		mentions[i] = strings.ToLower(mentions[i])
	}
	users := make([]*User, 0, len(mentions))

	if err := e.In("lower_name", mentions).Asc("lower_name").Find(&users); err != nil {
		return fmt.Errorf("find mentioned users: %v", err)
	}

	ids := make([]int64, 0, len(mentions))
	for _, user := range users {
		ids = append(ids, user.ID)
		if !user.IsOrganization() || user.NumMembers == 0 {
			continue
		}

		memberIDs := make([]int64, 0, user.NumMembers)
		orgUsers, err := GetOrgUsersByOrgID(user.ID)
		if err != nil {
			return fmt.Errorf("GetOrgUsersByOrgID [%d]: %v", user.ID, err)
		}

		for _, orgUser := range orgUsers {
			memberIDs = append(memberIDs, orgUser.ID)
		}

		ids = append(ids, memberIDs...)
	}

	if err := UpdateIssueUsersByMentions(e, issueID, ids); err != nil {
		return fmt.Errorf("UpdateIssueUsersByMentions: %v", err)
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
}

// Filter modes.
const (
	FilterModeAll = iota
	FilterModeAssign
	FilterModeCreate
	FilterModeMention
)

func parseCountResult(results []map[string][]byte) int64 {
	if len(results) == 0 {
		return 0
	}
	for _, result := range results[0] {
		return com.StrTo(string(result)).MustInt64()
	}
	return 0
}

// IssueStatsOptions contains parameters accepted by GetIssueStats.
type IssueStatsOptions struct {
	RepoID      int64
	Labels      string
	MilestoneID int64
	AssigneeID  int64
	MentionedID int64
	PosterID    int64
	IsPull      bool
	IssueIDs    []int64
}

// GetIssueStats returns issue statistic information by given conditions.
func GetIssueStats(opts *IssueStatsOptions) (*IssueStats, error) {
	stats := &IssueStats{}

	countSession := func(opts *IssueStatsOptions) *xorm.Session {
		sess := x.
			Where("issue.repo_id = ?", opts.RepoID).
			And("issue.is_pull = ?", opts.IsPull)

		if len(opts.IssueIDs) > 0 {
			sess.In("issue.id", opts.IssueIDs)
		}

		if len(opts.Labels) > 0 && opts.Labels != "0" {
			labelIDs, err := base.StringsToInt64s(strings.Split(opts.Labels, ","))
			if err != nil {
				log.Warn("Malformed Labels argument: %s", opts.Labels)
			} else if len(labelIDs) > 0 {
				sess.Join("INNER", "issue_label", "issue.id = issue_label.issue_id").
					In("issue_label.label_id", labelIDs)
			}
		}

		if opts.MilestoneID > 0 {
			sess.And("issue.milestone_id = ?", opts.MilestoneID)
		}

		if opts.AssigneeID > 0 {
			sess.And("issue.assignee_id = ?", opts.AssigneeID)
		}

		if opts.PosterID > 0 {
			sess.And("issue.poster_id = ?", opts.PosterID)
		}

		if opts.MentionedID > 0 {
			sess.Join("INNER", "issue_user", "issue.id = issue_user.issue_id").
				And("issue_user.uid = ?", opts.MentionedID).
				And("issue_user.is_mentioned = ?", true)
		}

		return sess
	}

	var err error
	stats.OpenCount, err = countSession(opts).
		And("issue.is_closed = ?", false).
		Count(new(Issue))
	if err != nil {
		return stats, err
	}
	stats.ClosedCount, err = countSession(opts).
		And("issue.is_closed = ?", true).
		Count(new(Issue))
	return stats, err
}

// GetUserIssueStats returns issue statistic information for dashboard by given conditions.
func GetUserIssueStats(repoID, uid int64, repoIDs []int64, filterMode int, isPull bool) *IssueStats {
	stats := &IssueStats{}

	countSession := func(isClosed, isPull bool, repoID int64, repoIDs []int64) *xorm.Session {
		sess := x.
			Where("issue.is_closed = ?", isClosed).
			And("issue.is_pull = ?", isPull)

		if repoID > 0 {
			sess.And("repo_id = ?", repoID)
		} else if len(repoIDs) > 0 {
			sess.In("repo_id", repoIDs)
		}

		return sess
	}

	stats.AssignCount, _ = countSession(false, isPull, repoID, nil).
		And("assignee_id = ?", uid).
		Count(new(Issue))

	stats.CreateCount, _ = countSession(false, isPull, repoID, nil).
		And("poster_id = ?", uid).
		Count(new(Issue))

	stats.YourRepositoriesCount, _ = countSession(false, isPull, repoID, repoIDs).
		Count(new(Issue))

	switch filterMode {
	case FilterModeAll:
		stats.OpenCount, _ = countSession(false, isPull, repoID, repoIDs).
			Count(new(Issue))
		stats.ClosedCount, _ = countSession(true, isPull, repoID, repoIDs).
			Count(new(Issue))
	case FilterModeAssign:
		stats.OpenCount, _ = countSession(false, isPull, repoID, nil).
			And("assignee_id = ?", uid).
			Count(new(Issue))
		stats.ClosedCount, _ = countSession(true, isPull, repoID, nil).
			And("assignee_id = ?", uid).
			Count(new(Issue))
	case FilterModeCreate:
		stats.OpenCount, _ = countSession(false, isPull, repoID, nil).
			And("poster_id = ?", uid).
			Count(new(Issue))
		stats.ClosedCount, _ = countSession(true, isPull, repoID, nil).
			And("poster_id = ?", uid).
			Count(new(Issue))
	}

	return stats
}

// GetRepoIssueStats returns number of open and closed repository issues by given filter mode.
func GetRepoIssueStats(repoID, uid int64, filterMode int, isPull bool) (numOpen int64, numClosed int64) {
	countSession := func(isClosed, isPull bool, repoID int64) *xorm.Session {
		sess := x.
			Where("is_closed = ?", isClosed).
			And("is_pull = ?", isPull).
			And("repo_id = ?", repoID)

		return sess
	}

	openCountSession := countSession(false, isPull, repoID)
	closedCountSession := countSession(true, isPull, repoID)

	switch filterMode {
	case FilterModeAssign:
		openCountSession.And("assignee_id = ?", uid)
		closedCountSession.And("assignee_id = ?", uid)
	case FilterModeCreate:
		openCountSession.And("poster_id = ?", uid)
		closedCountSession.And("poster_id = ?", uid)
	}

	openResult, _ := openCountSession.Count(new(Issue))
	closedResult, _ := closedCountSession.Count(new(Issue))

	return openResult, closedResult
}

func updateIssue(e Engine, issue *Issue) error {
	_, err := e.Id(issue.ID).AllCols().Update(issue)
	if err != nil {
		return err
	}
	UpdateIssueIndexer(issue)
	return nil
}

// UpdateIssue updates all fields of given issue.
func UpdateIssue(issue *Issue) error {
	return updateIssue(x, issue)
}
