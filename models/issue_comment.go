// Copyright 2018 The Gitea Authors.
// Copyright 2016 The Gogs Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"fmt"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"github.com/Unknwon/com"
	"github.com/go-xorm/builder"
	"github.com/go-xorm/xorm"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/util"
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
	// Added a due date
	CommentTypeAddedDeadline
	// Modified the due date
	CommentTypeModifiedDeadline
	// Removed a due date
	CommentTypeRemovedDeadline
	// Dependency added
	CommentTypeAddDependency
	//Dependency removed
	CommentTypeRemoveDependency
	// Comment a line of code
	CommentTypeCode
	// Reviews a pull request by giving general feedback
	CommentTypeReview
	// Lock an issue, giving only collaborators access
	CommentTypeLock
	// Unlocks a previously locked issue
	CommentTypeUnlock
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
	ID               int64 `xorm:"pk autoincr"`
	Type             CommentType
	PosterID         int64  `xorm:"INDEX"`
	Poster           *User  `xorm:"-"`
	IssueID          int64  `xorm:"INDEX"`
	Issue            *Issue `xorm:"-"`
	LabelID          int64
	Label            *Label `xorm:"-"`
	OldMilestoneID   int64
	MilestoneID      int64
	OldMilestone     *Milestone `xorm:"-"`
	Milestone        *Milestone `xorm:"-"`
	AssigneeID       int64
	RemovedAssignee  bool
	Assignee         *User `xorm:"-"`
	OldTitle         string
	NewTitle         string
	DependentIssueID int64
	DependentIssue   *Issue `xorm:"-"`

	CommitID        int64
	Line            int64 // - previous line / + proposed line
	TreePath        string
	Content         string `xorm:"TEXT"`
	RenderedContent string `xorm:"-"`

	// Path represents the 4 lines of code cemented by this comment
	Patch string `xorm:"TEXT"`

	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`

	// Reference issue in commit message
	CommitSHA string `xorm:"VARCHAR(40)"`

	Attachments []*Attachment `xorm:"-"`
	Reactions   ReactionList  `xorm:"-"`

	// For view issue page.
	ShowTag CommentTag `xorm:"-"`

	Review      *Review `xorm:"-"`
	ReviewID    int64
	Invalidated bool
}

// LoadIssue loads issue from database
func (c *Comment) LoadIssue() (err error) {
	if c.Issue != nil {
		return nil
	}
	c.Issue, err = GetIssueByID(c.IssueID)
	return
}

// AfterDelete is invoked from XORM after the object is deleted.
func (c *Comment) AfterDelete() {
	if c.ID <= 0 {
		return
	}

	_, err := DeleteAttachmentsByComment(c.ID, true)

	if err != nil {
		log.Info("Could not delete files for comment %d on issue #%d: %s", c.ID, c.IssueID, err)
	}
}

// HTMLURL formats a URL-string to the issue-comment
func (c *Comment) HTMLURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	if c.Type == CommentTypeCode {
		if c.ReviewID == 0 {
			return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
		}
		if c.Review == nil {
			if err := c.LoadReview(); err != nil {
				log.Warn("LoadReview(%d): %v", c.ReviewID, err)
				return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
			}
		}
		if c.Review.Type <= ReviewTypePending {
			return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
		}
	}
	return fmt.Sprintf("%s#%s", c.Issue.HTMLURL(), c.HashTag())
}

// IssueURL formats a URL-string to the issue
func (c *Comment) IssueURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}

	if c.Issue.IsPull {
		return ""
	}

	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return c.Issue.HTMLURL()
}

// PRURL formats a URL-string to the pull-request
func (c *Comment) PRURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}

	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}

	if !c.Issue.IsPull {
		return ""
	}
	return c.Issue.HTMLURL()
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
		Created:  c.CreatedUnix.AsTime(),
		Updated:  c.UpdatedUnix.AsTime(),
	}
}

// CommentHashTag returns unique hash tag for comment id.
func CommentHashTag(id int64) string {
	return fmt.Sprintf("issuecomment-%d", id)
}

// HashTag returns unique hash tag for comment.
func (c *Comment) HashTag() string {
	return CommentHashTag(c.ID)
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

// LoadPoster loads comment poster
func (c *Comment) LoadPoster() error {
	if c.PosterID <= 0 || c.Poster != nil {
		return nil
	}

	var err error
	c.Poster, err = getUserByID(x, c.PosterID)
	if err != nil {
		if IsErrUserNotExist(err) {
			c.PosterID = -1
			c.Poster = NewGhostUser()
		} else {
			log.Error(3, "getUserByID[%d]: %v", c.ID, err)
		}
	}
	return nil
}

// LoadAttachments loads attachments
func (c *Comment) LoadAttachments() error {
	if len(c.Attachments) > 0 {
		return nil
	}

	var err error
	c.Attachments, err = getAttachmentsByCommentID(x, c.ID)
	if err != nil {
		log.Error(3, "getAttachmentsByCommentID[%d]: %v", c.ID, err)
	}
	return nil
}

// LoadAssigneeUser if comment.Type is CommentTypeAssignees, then load assignees
func (c *Comment) LoadAssigneeUser() error {
	var err error

	if c.AssigneeID > 0 {
		c.Assignee, err = getUserByID(x, c.AssigneeID)
		if err != nil {
			if !IsErrUserNotExist(err) {
				return err
			}
			c.Assignee = NewGhostUser()
		}
	}
	return nil
}

// LoadDepIssueDetails loads Dependent Issue Details
func (c *Comment) LoadDepIssueDetails() (err error) {
	if c.DependentIssueID <= 0 || c.DependentIssue != nil {
		return nil
	}
	c.DependentIssue, err = getIssueByID(x, c.DependentIssueID)
	return err
}

// MailParticipants sends new comment emails to repository watchers
// and mentioned people.
func (c *Comment) MailParticipants(opType ActionType, issue *Issue) (err error) {
	return c.mailParticipants(x, opType, issue)
}

func (c *Comment) mailParticipants(e Engine, opType ActionType, issue *Issue) (err error) {
	mentions := markup.FindAllMentions(c.Content)
	if err = UpdateIssueMentions(e, c.IssueID, mentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", c.IssueID, err)
	}

	content := c.Content

	switch opType {
	case ActionCloseIssue:
		content = fmt.Sprintf("Closed #%d", issue.Index)
	case ActionReopenIssue:
		content = fmt.Sprintf("Reopened #%d", issue.Index)
	}
	if err = mailIssueCommentToParticipants(e, issue, c.Poster, content, c, mentions); err != nil {
		log.Error(4, "mailIssueCommentToParticipants: %v", err)
	}

	return nil
}

func (c *Comment) loadReactions(e Engine) (err error) {
	if c.Reactions != nil {
		return nil
	}
	c.Reactions, err = findReactions(e, FindReactionsOptions{
		IssueID:   c.IssueID,
		CommentID: c.ID,
	})
	if err != nil {
		return err
	}
	// Load reaction user data
	if _, err := c.Reactions.LoadUsers(); err != nil {
		return err
	}
	return nil
}

// LoadReactions loads comment reactions
func (c *Comment) LoadReactions() error {
	return c.loadReactions(x)
}

func (c *Comment) loadReview(e Engine) (err error) {
	if c.Review == nil {
		if c.Review, err = getReviewByID(e, c.ReviewID); err != nil {
			return err
		}
	}
	c.Review.Issue = c.Issue
	return nil
}

// LoadReview loads the associated review
func (c *Comment) LoadReview() error {
	return c.loadReview(x)
}

func (c *Comment) checkInvalidation(e Engine, doer *User, repo *git.Repository, branch string) error {
	// FIXME differentiate between previous and proposed line
	commit, err := repo.LineBlame(branch, repo.Path, c.TreePath, uint(c.UnsignedLine()))
	if err != nil {
		return err
	}
	if c.CommitSHA != "" && c.CommitSHA != commit.ID.String() {
		c.Invalidated = true
		return UpdateComment(doer, c, "")
	}
	return nil
}

// CheckInvalidation checks if the line of code comment got changed by another commit.
// If the line got changed the comment is going to be invalidated.
func (c *Comment) CheckInvalidation(repo *git.Repository, doer *User, branch string) error {
	return c.checkInvalidation(x, doer, repo, branch)
}

// DiffSide returns "previous" if Comment.Line is a LOC of the previous changes and "proposed" if it is a LOC of the proposed changes.
func (c *Comment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// UnsignedLine returns the LOC of the code comment without + or -
func (c *Comment) UnsignedLine() uint64 {
	if c.Line < 0 {
		return uint64(c.Line * -1)
	}
	return uint64(c.Line)
}

// AsDiff returns c.Patch as *Diff
func (c *Comment) AsDiff() (*Diff, error) {
	diff, err := ParsePatch(setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.Patch))
	if err != nil {
		return nil, err
	}
	if len(diff.Files) == 0 {
		return nil, fmt.Errorf("no file found for comment ID: %d", c.ID)
	}
	secs := diff.Files[0].Sections
	if len(secs) == 0 {
		return nil, fmt.Errorf("no sections found for comment ID: %d", c.ID)
	}
	return diff, nil
}

// MustAsDiff executes AsDiff and logs the error instead of returning
func (c *Comment) MustAsDiff() *Diff {
	diff, err := c.AsDiff()
	if err != nil {
		log.Warn("MustAsDiff: %v", err)
	}
	return diff
}

// CodeCommentURL returns the url to a comment in code
func (c *Comment) CodeCommentURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error(4, "loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
}

func createComment(e *xorm.Session, opts *CreateCommentOptions) (_ *Comment, err error) {
	var LabelID int64
	if opts.Label != nil {
		LabelID = opts.Label.ID
	}

	comment := &Comment{
		Type:             opts.Type,
		PosterID:         opts.Doer.ID,
		Poster:           opts.Doer,
		IssueID:          opts.Issue.ID,
		LabelID:          LabelID,
		OldMilestoneID:   opts.OldMilestoneID,
		MilestoneID:      opts.MilestoneID,
		RemovedAssignee:  opts.RemovedAssignee,
		AssigneeID:       opts.AssigneeID,
		CommitID:         opts.CommitID,
		CommitSHA:        opts.CommitSHA,
		Line:             opts.LineNum,
		Content:          opts.Content,
		OldTitle:         opts.OldTitle,
		NewTitle:         opts.NewTitle,
		DependentIssueID: opts.DependentIssueID,
		TreePath:         opts.TreePath,
		ReviewID:         opts.ReviewID,
		Patch:            opts.Patch,
	}
	if _, err = e.Insert(comment); err != nil {
		return nil, err
	}

	if err = opts.Repo.getOwner(e); err != nil {
		return nil, err
	}

	if err = sendCreateCommentAction(e, opts, comment); err != nil {
		return nil, err
	}

	return comment, nil
}

func sendCreateCommentAction(e *xorm.Session, opts *CreateCommentOptions, comment *Comment) (err error) {
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
	case CommentTypeCode:
		if comment.ReviewID != 0 {
			if comment.Review == nil {
				if err := comment.loadReview(e); err != nil {
					return err
				}
			}
			if comment.Review.Type <= ReviewTypePending {
				return nil
			}
		}
		fallthrough
	case CommentTypeComment:
		act.OpType = ActionCommentIssue

		if _, err = e.Exec("UPDATE `issue` SET num_comments=num_comments+1 WHERE id=?", opts.Issue.ID); err != nil {
			return err
		}

		// Check attachments
		attachments := make([]*Attachment, 0, len(opts.Attachments))
		for _, uuid := range opts.Attachments {
			attach, err := getAttachmentByUUID(e, uuid)
			if err != nil {
				if IsErrAttachmentNotExist(err) {
					continue
				}
				return fmt.Errorf("getAttachmentByUUID [%s]: %v", uuid, err)
			}
			attachments = append(attachments, attach)
		}

		for i := range attachments {
			attachments[i].IssueID = opts.Issue.ID
			attachments[i].CommentID = comment.ID
			// No assign value could be 0, so ignore AllCols().
			if _, err = e.ID(attachments[i].ID).Update(attachments[i]); err != nil {
				return fmt.Errorf("update attachment [%d]: %v", attachments[i].ID, err)
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
			return err
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
			return err
		}
	}
	// update the issue's updated_unix column
	if err = updateIssueCols(e, opts.Issue, "updated_unix"); err != nil {
		return err
	}
	// Notify watchers for whatever action comes in, ignore if no action type.
	if act.OpType > 0 {
		if err = notifyWatchers(e, act); err != nil {
			log.Error(4, "notifyWatchers: %v", err)
		}
	}
	return nil
}

func createStatusComment(e *xorm.Session, doer *User, issue *Issue) (*Comment, error) {
	cmtType := CommentTypeClose
	if !issue.IsClosed {
		cmtType = CommentTypeReopen
	}
	return createComment(e, &CreateCommentOptions{
		Type:  cmtType,
		Doer:  doer,
		Repo:  issue.Repo,
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

func createAssigneeComment(e *xorm.Session, doer *User, repo *Repository, issue *Issue, assigneeID int64, removedAssignee bool) (*Comment, error) {
	return createComment(e, &CreateCommentOptions{
		Type:            CommentTypeAssignees,
		Doer:            doer,
		Repo:            repo,
		Issue:           issue,
		RemovedAssignee: removedAssignee,
		AssigneeID:      assigneeID,
	})
}

func createDeadlineComment(e *xorm.Session, doer *User, issue *Issue, newDeadlineUnix util.TimeStamp) (*Comment, error) {

	var content string
	var commentType CommentType

	// newDeadline = 0 means deleting
	if newDeadlineUnix == 0 {
		commentType = CommentTypeRemovedDeadline
		content = issue.DeadlineUnix.Format("2006-01-02")
	} else if issue.DeadlineUnix == 0 {
		// Check if the new date was added or modified
		// If the actual deadline is 0 => deadline added
		commentType = CommentTypeAddedDeadline
		content = newDeadlineUnix.Format("2006-01-02")
	} else { // Otherwise modified
		commentType = CommentTypeModifiedDeadline
		content = newDeadlineUnix.Format("2006-01-02") + "|" + issue.DeadlineUnix.Format("2006-01-02")
	}

	if err := issue.loadRepo(e); err != nil {
		return nil, err
	}

	return createComment(e, &CreateCommentOptions{
		Type:    commentType,
		Doer:    doer,
		Repo:    issue.Repo,
		Issue:   issue,
		Content: content,
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

// Creates issue dependency comment
func createIssueDependencyComment(e *xorm.Session, doer *User, issue *Issue, dependentIssue *Issue, add bool) (err error) {
	cType := CommentTypeAddDependency
	if !add {
		cType = CommentTypeRemoveDependency
	}
	if err = issue.loadRepo(e); err != nil {
		return
	}

	// Make two comments, one in each issue
	_, err = createComment(e, &CreateCommentOptions{
		Type:             cType,
		Doer:             doer,
		Repo:             issue.Repo,
		Issue:            issue,
		DependentIssueID: dependentIssue.ID,
	})
	if err != nil {
		return
	}

	_, err = createComment(e, &CreateCommentOptions{
		Type:             cType,
		Doer:             doer,
		Repo:             issue.Repo,
		Issue:            dependentIssue,
		DependentIssueID: issue.ID,
	})
	if err != nil {
		return
	}

	return
}

// CreateCommentOptions defines options for creating comment
type CreateCommentOptions struct {
	Type  CommentType
	Doer  *User
	Repo  *Repository
	Issue *Issue
	Label *Label

	DependentIssueID int64
	OldMilestoneID   int64
	MilestoneID      int64
	AssigneeID       int64
	RemovedAssignee  bool
	OldTitle         string
	NewTitle         string
	CommitID         int64
	CommitSHA        string
	Patch            string
	LineNum          int64
	TreePath         string
	ReviewID         int64
	Content          string
	Attachments      []string // UUIDs of attachments
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

	if err = sess.Commit(); err != nil {
		return nil, err
	}

	return comment, nil
}

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(doer *User, repo *Repository, issue *Issue, content string, attachments []string) (*Comment, error) {
	comment, err := CreateComment(&CreateCommentOptions{
		Type:        CommentTypeComment,
		Doer:        doer,
		Repo:        repo,
		Issue:       issue,
		Content:     content,
		Attachments: attachments,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateComment: %v", err)
	}

	mode, _ := AccessLevel(doer, repo)
	if err = PrepareWebhooks(repo, HookEventIssueComment, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentCreated,
		Issue:      issue.APIFormat(),
		Comment:    comment.APIFormat(),
		Repository: repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	} else {
		go HookQueue.Add(repo.ID)
	}
	return comment, nil
}

// CreateCodeComment creates a plain code comment at the specified line / path
func CreateCodeComment(doer *User, repo *Repository, issue *Issue, content, treePath string, line, reviewID int64) (*Comment, error) {
	var commitID, patch string
	pr, err := GetPullRequestByIssueID(issue.ID)
	if err != nil {
		return nil, fmt.Errorf("GetPullRequestByIssueID: %v", err)
	}
	if err := pr.GetBaseRepo(); err != nil {
		return nil, fmt.Errorf("GetHeadRepo: %v", err)
	}
	gitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}

	// FIXME validate treePath
	// Get latest commit referencing the commented line
	// No need for get commit for base branch changes
	if line > 0 {
		commit, err := gitRepo.LineBlame(pr.GetGitRefName(), gitRepo.Path, treePath, uint(line))
		if err == nil {
			commitID = commit.ID.String()
		} else if err != nil && !strings.Contains(err.Error(), "exit status 128 - fatal: no such path") {
			return nil, fmt.Errorf("LineBlame[%s, %s, %s, %d]: %v", pr.GetGitRefName(), gitRepo.Path, treePath, line, err)
		}
	}

	// Only fetch diff if comment is review comment
	if reviewID != 0 {
		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return nil, fmt.Errorf("GetRefCommitID[%s]: %v", pr.GetGitRefName(), err)
		}
		patchBuf := new(bytes.Buffer)
		if err := GetRawDiffForFile(gitRepo.Path, pr.MergeBase, headCommitID, RawDiffNormal, treePath, patchBuf); err != nil {
			return nil, fmt.Errorf("GetRawDiffForLine[%s, %s, %s, %s]: %v", err, gitRepo.Path, pr.MergeBase, headCommitID, treePath)
		}
		patch = CutDiffAroundLine(strings.NewReader(patchBuf.String()), int64((&Comment{Line: line}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines)
	}
	return CreateComment(&CreateCommentOptions{
		Type:      CommentTypeCode,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		Content:   content,
		LineNum:   line,
		TreePath:  treePath,
		CommitSHA: commitID,
		ReviewID:  reviewID,
		Patch:     patch,
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
	has, err := x.ID(id).Get(c)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{id, 0}
	}
	return c, nil
}

// FindCommentsOptions describes the conditions to Find comments
type FindCommentsOptions struct {
	RepoID   int64
	IssueID  int64
	ReviewID int64
	Since    int64
	Type     CommentType
}

func (opts *FindCommentsOptions) toConds() builder.Cond {
	var cond = builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"issue.repo_id": opts.RepoID})
	}
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"comment.issue_id": opts.IssueID})
	}
	if opts.ReviewID > 0 {
		cond = cond.And(builder.Eq{"comment.review_id": opts.ReviewID})
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
		Asc("comment.id").
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
func UpdateComment(doer *User, c *Comment, oldContent string) error {
	if _, err := x.ID(c.ID).AllCols().Update(c); err != nil {
		return err
	}

	if err := c.LoadPoster(); err != nil {
		return err
	}
	if err := c.LoadIssue(); err != nil {
		return err
	}

	if err := c.Issue.LoadAttributes(); err != nil {
		return err
	}

	mode, _ := AccessLevel(doer, c.Issue.Repo)
	if err := PrepareWebhooks(c.Issue.Repo, HookEventIssueComment, &api.IssueCommentPayload{
		Action:  api.HookIssueCommentEdited,
		Issue:   c.Issue.APIFormat(),
		Comment: c.APIFormat(),
		Changes: &api.ChangesPayload{
			Body: &api.ChangesFromPayload{
				From: oldContent,
			},
		},
		Repository: c.Issue.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [comment_id: %d]: %v", c.ID, err)
	} else {
		go HookQueue.Add(c.Issue.Repo.ID)
	}

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(doer *User, comment *Comment) error {
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

	if err := sess.Commit(); err != nil {
		return err
	}

	if err := comment.LoadPoster(); err != nil {
		return err
	}
	if err := comment.LoadIssue(); err != nil {
		return err
	}

	if err := comment.Issue.LoadAttributes(); err != nil {
		return err
	}

	mode, _ := AccessLevel(doer, comment.Issue.Repo)

	if err := PrepareWebhooks(comment.Issue.Repo, HookEventIssueComment, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentDeleted,
		Issue:      comment.Issue.APIFormat(),
		Comment:    comment.APIFormat(),
		Repository: comment.Issue.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	} else {
		go HookQueue.Add(comment.Issue.Repo.ID)
	}

	return nil
}

// CodeComments represents comments on code by using this structure: FILENAME -> LINE (+ == proposed; - == previous) -> COMMENTS
type CodeComments map[string]map[int64][]*Comment

func fetchCodeComments(e Engine, issue *Issue, currentUser *User) (CodeComments, error) {
	return fetchCodeCommentsByReview(e, issue, currentUser, nil)
}

func fetchCodeCommentsByReview(e Engine, issue *Issue, currentUser *User, review *Review) (CodeComments, error) {
	pathToLineToComment := make(CodeComments)
	if review == nil {
		review = &Review{ID: 0}
	}
	//Find comments
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  issue.ID,
		ReviewID: review.ID,
	}
	conds := opts.toConds()
	if review.ID == 0 {
		conds = conds.And(builder.Eq{"invalidated": false})
	}

	var comments []*Comment
	if err := e.Where(conds).
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := CommentList(comments).loadPosters(e); err != nil {
		return nil, err
	}

	if err := issue.loadRepo(e); err != nil {
		return nil, err
	}
	// Find all reviews by ReviewID
	reviews := make(map[int64]*Review)
	var ids = make([]int64, 0, len(comments))
	for _, comment := range comments {
		if comment.ReviewID != 0 {
			ids = append(ids, comment.ReviewID)
		}
	}
	if err := e.In("id", ids).Find(&reviews); err != nil {
		return nil, err
	}
	for _, comment := range comments {
		if re, ok := reviews[comment.ReviewID]; ok && re != nil {
			// If the review is pending only the author can see the comments (except the review is set)
			if review.ID == 0 {
				if re.Type == ReviewTypePending &&
					(currentUser == nil || currentUser.ID != re.ReviewerID) {
					continue
				}
			}
			comment.Review = re
		}

		comment.RenderedContent = string(markdown.Render([]byte(comment.Content), issue.Repo.Link(),
			issue.Repo.ComposeMetas()))
		if pathToLineToComment[comment.TreePath] == nil {
			pathToLineToComment[comment.TreePath] = make(map[int64][]*Comment)
		}
		pathToLineToComment[comment.TreePath][comment.Line] = append(pathToLineToComment[comment.TreePath][comment.Line], comment)
	}
	return pathToLineToComment, nil
}

// FetchCodeComments will return a 2d-map: ["Path"]["Line"] = Comments at line
func FetchCodeComments(issue *Issue, currentUser *User) (CodeComments, error) {
	return fetchCodeComments(x, issue, currentUser)
}
