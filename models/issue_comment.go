// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markdown"
)

// CommentType defines whether a comment is just a simple comment, an action (like close) or a reference.
type CommentType int

// define unknown comment type
const (
	CommentTypeUnknown CommentType = -1
)

// Enumerate all the comment types
const (
	// Plain comment, can be associated with a commit (CommitID > 0) and a line (LineNum > 0)
	CommentTypeComment CommentType = iota
	CommentTypeReopen
	CommentTypeClose

	// References.
	CommentTypeIssueRef
	// Reference from a commit (not part of a pull request)
	CommentTypeCommitRef
	// Reference from a comment
	CommentTypeCommentRef
	// Reference from a pull request
	CommentTypePullRef
	// Labels changed
	CommentTypeLabel
	// Milestone changed
	CommentTypeMilestone
	// Assignees changed
	CommentTypeAssignees
	// Change Title
	CommentTypeChangeTitle
	// Delete Branch
	CommentTypeDeleteBranch
	// Start a stopwatch for time tracking
	CommentTypeStartTracking
	// Stop a stopwatch for time tracking
	CommentTypeStopTracking
	// Add time manual for time tracking
	CommentTypeAddTimeManual
	// Cancel a stopwatch for time tracking
	CommentTypeCancelTracking
)

// CommentTag defines comment tag type
type CommentTag int

// Enumerate all the comment tag types
const (
	CommentTagNone CommentTag = iota
	CommentTagPoster
	CommentTagWriter
	CommentTagOwner
)

// Comment represents a comment in commit and issue page.
type Comment struct {
	ID             int64 `xorm:"pk autoincr"`
	Type           CommentType
	PosterID       int64 `xorm:"INDEX"`
	Poster         *User `xorm:"-"`
	IssueID        int64 `xorm:"INDEX"`
	LabelID        int64
	Label          *Label `xorm:"-"`
	OldMilestoneID int64
	MilestoneID    int64
	OldMilestone   *Milestone `xorm:"-"`
	Milestone      *Milestone `xorm:"-"`
	OldAssigneeID  int64
	AssigneeID     int64
	Assignee       *User `xorm:"-"`
	OldAssignee    *User `xorm:"-"`
	OldTitle       string
	NewTitle       string

	CommitID        int64
	Line            int64
	Content         string `xorm:"TEXT"`
	RenderedContent string `xorm:"-"`

	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"INDEX created"`
	Updated     time.Time `xorm:"-"`
	UpdatedUnix int64     `xorm:"INDEX updated"`

	// Reference issue in commit message
	CommitSHA string `xorm:"VARCHAR(40)"`

	Attachments []*Attachment `xorm:"-"`

	// For view issue page.
	ShowTag CommentTag `xorm:"-"`
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (c *Comment) AfterSet(colName string, _ xorm.Cell) {
	var err error
	switch colName {
	case "id":
		c.Attachments, err = GetAttachmentsByCommentID(c.ID)
		if err != nil {
			log.Error(3, "GetAttachmentsByCommentID[%d]: %v", c.ID, err)
		}

	case "poster_id":
		c.Poster, err = GetUserByID(c.PosterID)
		if err != nil {
			if IsErrUserNotExist(err) {
				c.PosterID = -1
				c.Poster = NewGhostUser()
			} else {
				log.Error(3, "GetUserByID[%d]: %v", c.ID, err)
			}
		}
	case "created_unix":
		c.Created = time.Unix(c.CreatedUnix, 0).Local()
	case "updated_unix":
		c.Updated = time.Unix(c.UpdatedUnix, 0).Local()
	}
}

// AfterDelete is invoked from XORM after the object is deleted.
func (c *Comment) AfterDelete() {
	_, err := DeleteAttachmentsByComment(c.ID, true)

	if err != nil {
		log.Info("Could not delete files for comment %d on issue #%d: %s", c.ID, c.IssueID, err)
	}
}

// HTMLURL formats a URL-string to the issue-comment
func (c *Comment) HTMLURL() string {
	issue, err := GetIssueByID(c.IssueID)
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "GetIssueByID(%d): %v", c.IssueID, err)
		return ""
	}
	return fmt.Sprintf("%s#%s", issue.HTMLURL(), c.HashTag())
}

// IssueURL formats a URL-string to the issue
func (c *Comment) IssueURL() string {
	issue, err := GetIssueByID(c.IssueID)
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "GetIssueByID(%d): %v", c.IssueID, err)
		return ""
	}

	if issue.IsPull {
		return ""
	}
	return issue.HTMLURL()
}

// PRURL formats a URL-string to the pull-request
func (c *Comment) PRURL() string {
	issue, err := GetIssueByID(c.IssueID)
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "GetIssueByID(%d): %v", c.IssueID, err)
		return ""
	}

	if !issue.IsPull {
		return ""
	}
	return issue.HTMLURL()
}

// APIFormat converts a Comment to the api.Comment format
func (c *Comment) APIFormat() *api.Comment {
	return &api.Comment{
		ID:       c.ID,
		Poster:   c.Poster.APIFormat(),
		HTMLURL:  c.HTMLURL(),
		IssueURL: c.IssueURL(),
		PRURL:    c.PRURL(),
		Body:     c.Content,
		Created:  c.Created,
		Updated:  c.Updated,
	}
}

// HashTag returns unique hash tag for comment.
func (c *Comment) HashTag() string {
	return "issuecomment-" + com.ToStr(c.ID)
}

// EventTag returns unique event hash tag for comment.
func (c *Comment) EventTag() string {
	return "event-" + com.ToStr(c.ID)
}

// LoadLabel if comment.Type is CommentTypeLabel, then load Label
func (c *Comment) LoadLabel() error {
	var label Label
	has, err := x.ID(c.LabelID).Get(&label)
	if err != nil {
		return err
	} else if has {
		c.Label = &label
	} else {
		// Ignore Label is deleted, but not clear this table
		log.Warn("Commit %d cannot load label %d", c.ID, c.LabelID)
	}

	return nil
}

// LoadMilestone if comment.Type is CommentTypeMilestone, then load milestone
func (c *Comment) LoadMilestone() error {
	if c.OldMilestoneID > 0 {
		var oldMilestone Milestone
		has, err := x.ID(c.OldMilestoneID).Get(&oldMilestone)
		if err != nil {
			return err
		} else if has {
			c.OldMilestone = &oldMilestone
		}
	}

	if c.MilestoneID > 0 {
		var milestone Milestone
		has, err := x.ID(c.MilestoneID).Get(&milestone)
		if err != nil {
			return err
		} else if has {
			c.Milestone = &milestone
		}
	}
	return nil
}

// LoadAssignees if comment.Type is CommentTypeAssignees, then load assignees
func (c *Comment) LoadAssignees() error {
	var err error
	if c.OldAssigneeID > 0 {
		c.OldAssignee, err = getUserByID(x, c.OldAssigneeID)
		if err != nil {
			return err
		}
	}

	if c.AssigneeID > 0 {
		c.Assignee, err = getUserByID(x, c.AssigneeID)
		if err != nil {
			return err
		}
	}
	return nil
}

// MailParticipants sends new comment emails to repository watchers
// and mentioned people.
func (c *Comment) MailParticipants(e Engine, opType ActionType, issue *Issue) (err error) {
	mentions := markdown.FindAllMentions(c.Content)
	if err = UpdateIssueMentions(e, c.IssueID, mentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", c.IssueID, err)
	}

	switch opType {
	case ActionCommentIssue:
		issue.Content = c.Content
	case ActionCloseIssue:
		issue.Content = fmt.Sprintf("Closed #%d", issue.Index)
	case ActionReopenIssue:
		issue.Content = fmt.Sprintf("Reopened #%d", issue.Index)
	}
	if err = mailIssueCommentToParticipants(e, issue, c.Poster, c, mentions); err != nil {
		log.Error(4, "mailIssueCommentToParticipants: %v", err)
	}

	return nil
}

func createComment(e *xorm.Session, opts *CreateCommentOptions) (_ *Comment, err error) {
	var LabelID int64
	if opts.Label != nil {
		LabelID = opts.Label.ID
	}
	comment := &Comment{
		Type:           opts.Type,
		PosterID:       opts.Doer.ID,
		Poster:         opts.Doer,
		IssueID:        opts.Issue.ID,
		LabelID:        LabelID,
		OldMilestoneID: opts.OldMilestoneID,
		MilestoneID:    opts.MilestoneID,
		OldAssigneeID:  opts.OldAssigneeID,
		AssigneeID:     opts.AssigneeID,
		CommitID:       opts.CommitID,
		CommitSHA:      opts.CommitSHA,
		Line:           opts.LineNum,
		Content:        opts.Content,
		OldTitle:       opts.OldTitle,
		NewTitle:       opts.NewTitle,
	}
	if _, err = e.Insert(comment); err != nil {
		return nil, err
	}

	if err = opts.Repo.getOwner(e); err != nil {
		return nil, err
	}

	// Compose comment action, could be plain comment, close or reopen issue/pull request.
	// This object will be used to notify watchers in the end of function.
	act := &Action{
		ActUserID: opts.Doer.ID,
		ActUser:   opts.Doer,
		Content:   fmt.Sprintf("%d|%s", opts.Issue.Index, strings.Split(opts.Content, "\n")[0]),
		RepoID:    opts.Repo.ID,
		Repo:      opts.Repo,
		Comment:   comment,
		CommentID: comment.ID,
		IsPrivate: opts.Repo.IsPrivate,
	}

	// Check comment type.
	switch opts.Type {
	case CommentTypeComment:
		act.OpType = ActionCommentIssue

		if _, err = e.Exec("UPDATE `issue` SET num_comments=num_comments+1 WHERE id=?", opts.Issue.ID); err != nil {
			return nil, err
		}

		// Check attachments
		attachments := make([]*Attachment, 0, len(opts.Attachments))
		for _, uuid := range opts.Attachments {
			attach, err := getAttachmentByUUID(e, uuid)
			if err != nil {
				if IsErrAttachmentNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("getAttachmentByUUID [%s]: %v", uuid, err)
			}
			attachments = append(attachments, attach)
		}

		for i := range attachments {
			attachments[i].IssueID = opts.Issue.ID
			attachments[i].CommentID = comment.ID
			// No assign value could be 0, so ignore AllCols().
			if _, err = e.Id(attachments[i].ID).Update(attachments[i]); err != nil {
				return nil, fmt.Errorf("update attachment [%d]: %v", attachments[i].ID, err)
			}
		}

	case CommentTypeReopen:
		act.OpType = ActionReopenIssue
		if opts.Issue.IsPull {
			act.OpType = ActionReopenPullRequest
		}

		if opts.Issue.IsPull {
			_, err = e.Exec("UPDATE `repository` SET num_closed_pulls=num_closed_pulls-1 WHERE id=?", opts.Repo.ID)
		} else {
			_, err = e.Exec("UPDATE `repository` SET num_closed_issues=num_closed_issues-1 WHERE id=?", opts.Repo.ID)
		}
		if err != nil {
			return nil, err
		}

	case CommentTypeClose:
		act.OpType = ActionCloseIssue
		if opts.Issue.IsPull {
			act.OpType = ActionClosePullRequest
		}

		if opts.Issue.IsPull {
			_, err = e.Exec("UPDATE `repository` SET num_closed_pulls=num_closed_pulls+1 WHERE id=?", opts.Repo.ID)
		} else {
			_, err = e.Exec("UPDATE `repository` SET num_closed_issues=num_closed_issues+1 WHERE id=?", opts.Repo.ID)
		}
		if err != nil {
			return nil, err
		}
	}

	// update the issue's updated_unix column
	if err = updateIssueCols(e, opts.Issue); err != nil {
		return nil, err
	}

	// Notify watchers for whatever action comes in, ignore if no action type.
	if act.OpType > 0 {
		if err = notifyWatchers(e, act); err != nil {
			log.Error(4, "notifyWatchers: %v", err)
		}
		if err = comment.MailParticipants(e, act.OpType, opts.Issue); err != nil {
			log.Error(4, "MailParticipants: %v", err)
		}
	}

	return comment, nil
}

func createStatusComment(e *xorm.Session, doer *User, repo *Repository, issue *Issue) (*Comment, error) {
	cmtType := CommentTypeClose
	if !issue.IsClosed {
		cmtType = CommentTypeReopen
	}
	return createComment(e, &CreateCommentOptions{
		Type:  cmtType,
		Doer:  doer,
		Repo:  repo,
		Issue: issue,
	})
}

func createLabelComment(e *xorm.Session, doer *User, repo *Repository, issue *Issue, label *Label, add bool) (*Comment, error) {
	var content string
	if add {
		content = "1"
	}
	return createComment(e, &CreateCommentOptions{
		Type:    CommentTypeLabel,
		Doer:    doer,
		Repo:    repo,
		Issue:   issue,
		Label:   label,
		Content: content,
	})
}

func createMilestoneComment(e *xorm.Session, doer *User, repo *Repository, issue *Issue, oldMilestoneID, milestoneID int64) (*Comment, error) {
	return createComment(e, &CreateCommentOptions{
		Type:           CommentTypeMilestone,
		Doer:           doer,
		Repo:           repo,
		Issue:          issue,
		OldMilestoneID: oldMilestoneID,
		MilestoneID:    milestoneID,
	})
}

func createAssigneeComment(e *xorm.Session, doer *User, repo *Repository, issue *Issue, oldAssigneeID, assigneeID int64) (*Comment, error) {
	return createComment(e, &CreateCommentOptions{
		Type:          CommentTypeAssignees,
		Doer:          doer,
		Repo:          repo,
		Issue:         issue,
		OldAssigneeID: oldAssigneeID,
		AssigneeID:    assigneeID,
	})
}

func createChangeTitleComment(e *xorm.Session, doer *User, repo *Repository, issue *Issue, oldTitle, newTitle string) (*Comment, error) {
	return createComment(e, &CreateCommentOptions{
		Type:     CommentTypeChangeTitle,
		Doer:     doer,
		Repo:     repo,
		Issue:    issue,
		OldTitle: oldTitle,
		NewTitle: newTitle,
	})
}

func createDeleteBranchComment(e *xorm.Session, doer *User, repo *Repository, issue *Issue, branchName string) (*Comment, error) {
	return createComment(e, &CreateCommentOptions{
		Type:      CommentTypeDeleteBranch,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		CommitSHA: branchName,
	})
}

// CreateCommentOptions defines options for creating comment
type CreateCommentOptions struct {
	Type  CommentType
	Doer  *User
	Repo  *Repository
	Issue *Issue
	Label *Label

	OldMilestoneID int64
	MilestoneID    int64
	OldAssigneeID  int64
	AssigneeID     int64
	OldTitle       string
	NewTitle       string
	CommitID       int64
	CommitSHA      string
	LineNum        int64
	Content        string
	Attachments    []string // UUIDs of attachments
}

// CreateComment creates comment of issue or commit.
func CreateComment(opts *CreateCommentOptions) (comment *Comment, err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	comment, err = createComment(sess, opts)
	if err != nil {
		return nil, err
	}

	return comment, sess.Commit()
}

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(doer *User, repo *Repository, issue *Issue, content string, attachments []string) (*Comment, error) {
	return CreateComment(&CreateCommentOptions{
		Type:        CommentTypeComment,
		Doer:        doer,
		Repo:        repo,
		Issue:       issue,
		Content:     content,
		Attachments: attachments,
	})
}

// CreateRefComment creates a commit reference comment to issue.
func CreateRefComment(doer *User, repo *Repository, issue *Issue, content, commitSHA string) error {
	if len(commitSHA) == 0 {
		return fmt.Errorf("cannot create reference with empty commit SHA")
	}

	// Check if same reference from same commit has already existed.
	has, err := x.Get(&Comment{
		Type:      CommentTypeCommitRef,
		IssueID:   issue.ID,
		CommitSHA: commitSHA,
	})
	if err != nil {
		return fmt.Errorf("check reference comment: %v", err)
	} else if has {
		return nil
	}

	_, err = CreateComment(&CreateCommentOptions{
		Type:      CommentTypeCommitRef,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		CommitSHA: commitSHA,
		Content:   content,
	})
	return err
}

// GetCommentByID returns the comment by given ID.
func GetCommentByID(id int64) (*Comment, error) {
	c := new(Comment)
	has, err := x.Id(id).Get(c)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{id, 0}
	}
	return c, nil
}

// FindCommentsOptions describes the conditions to Find comments
type FindCommentsOptions struct {
	RepoID  int64
	IssueID int64
	Since   int64
	Type    CommentType
}

func (opts *FindCommentsOptions) toConds() builder.Cond {
	var cond = builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"issue.repo_id": opts.RepoID})
	}
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"comment.issue_id": opts.IssueID})
	}
	if opts.Since > 0 {
		cond = cond.And(builder.Gte{"comment.updated_unix": opts.Since})
	}
	if opts.Type != CommentTypeUnknown {
		cond = cond.And(builder.Eq{"comment.type": opts.Type})
	}
	return cond
}

func findComments(e Engine, opts FindCommentsOptions) ([]*Comment, error) {
	comments := make([]*Comment, 0, 10)
	sess := e.Where(opts.toConds())
	if opts.RepoID > 0 {
		sess.Join("INNER", "issue", "issue.id = comment.issue_id")
	}
	return comments, sess.
		Asc("comment.created_unix").
		Find(&comments)
}

// FindComments returns all comments according options
func FindComments(opts FindCommentsOptions) ([]*Comment, error) {
	return findComments(x, opts)
}

// GetCommentsByIssueID returns all comments of an issue.
func GetCommentsByIssueID(issueID int64) ([]*Comment, error) {
	return findComments(x, FindCommentsOptions{
		IssueID: issueID,
		Type:    CommentTypeUnknown,
	})
}

// GetCommentsByIssueIDSince returns a list of comments of an issue since a given time point.
func GetCommentsByIssueIDSince(issueID, since int64) ([]*Comment, error) {
	return findComments(x, FindCommentsOptions{
		IssueID: issueID,
		Type:    CommentTypeUnknown,
		Since:   since,
	})
}

// GetCommentsByRepoIDSince returns a list of comments for all issues in a repo since a given time point.
func GetCommentsByRepoIDSince(repoID, since int64) ([]*Comment, error) {
	return findComments(x, FindCommentsOptions{
		RepoID: repoID,
		Type:   CommentTypeUnknown,
		Since:  since,
	})
}

// UpdateComment updates information of comment.
func UpdateComment(c *Comment) error {
	_, err := x.Id(c.ID).AllCols().Update(c)
	return err
}

// DeleteComment deletes the comment
func DeleteComment(comment *Comment) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Delete(&Comment{
		ID: comment.ID,
	}); err != nil {
		return err
	}

	if comment.Type == CommentTypeComment {
		if _, err := sess.Exec("UPDATE `issue` SET num_comments = num_comments - 1 WHERE id = ?", comment.IssueID); err != nil {
			return err
		}
	}
	if _, err := sess.Where("comment_id = ?", comment.ID).Cols("is_deleted").Update(&Action{IsDeleted: true}); err != nil {
		return err
	}

	return sess.Commit()
}
