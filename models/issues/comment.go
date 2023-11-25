// Copyright 2018 The Gitea Authors.
// Copyright 2016 The Gogs Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strconv"
	"unicode/utf8"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// ErrCommentNotExist represents a "CommentNotExist" kind of error.
type ErrCommentNotExist struct {
	ID      int64
	IssueID int64
}

// IsErrCommentNotExist checks if an error is a ErrCommentNotExist.
func IsErrCommentNotExist(err error) bool {
	_, ok := err.(ErrCommentNotExist)
	return ok
}

func (err ErrCommentNotExist) Error() string {
	return fmt.Sprintf("comment does not exist [id: %d, issue_id: %d]", err.ID, err.IssueID)
}

func (err ErrCommentNotExist) Unwrap() error {
	return util.ErrNotExist
}

// CommentType defines whether a comment is just a simple comment, an action (like close) or a reference.
type CommentType int

// CommentTypeUndefined is used to search for comments of any type
const CommentTypeUndefined CommentType = -1

const (
	CommentTypeComment CommentType = iota // 0 Plain comment, can be associated with a commit (CommitID > 0) and a line (LineNum > 0)

	CommentTypeReopen // 1
	CommentTypeClose  // 2

	CommentTypeIssueRef   // 3 References.
	CommentTypeCommitRef  // 4 Reference from a commit (not part of a pull request)
	CommentTypeCommentRef // 5 Reference from a comment
	CommentTypePullRef    // 6 Reference from a pull request

	CommentTypeLabel        // 7 Labels changed
	CommentTypeMilestone    // 8 Milestone changed
	CommentTypeAssignees    // 9 Assignees changed
	CommentTypeChangeTitle  // 10 Change Title
	CommentTypeDeleteBranch // 11 Delete Branch

	CommentTypeStartTracking    // 12 Start a stopwatch for time tracking
	CommentTypeStopTracking     // 13 Stop a stopwatch for time tracking
	CommentTypeAddTimeManual    // 14 Add time manual for time tracking
	CommentTypeCancelTracking   // 15 Cancel a stopwatch for time tracking
	CommentTypeAddedDeadline    // 16 Added a due date
	CommentTypeModifiedDeadline // 17 Modified the due date
	CommentTypeRemovedDeadline  // 18 Removed a due date

	CommentTypeAddDependency    // 19 Dependency added
	CommentTypeRemoveDependency // 20 Dependency removed

	CommentTypeCode   // 21 Comment a line of code
	CommentTypeReview // 22 Reviews a pull request by giving general feedback

	CommentTypeLock   // 23 Lock an issue, giving only collaborators access
	CommentTypeUnlock // 24 Unlocks a previously locked issue

	CommentTypeChangeTargetBranch // 25 Change pull request's target branch

	CommentTypeDeleteTimeManual // 26 Delete time manual for time tracking

	CommentTypeReviewRequest   // 27 add or remove Request from one
	CommentTypeMergePull       // 28 merge pull request
	CommentTypePullRequestPush // 29 push to PR head branch

	CommentTypeProject      // 30 Project changed
	CommentTypeProjectBoard // 31 Project board changed

	CommentTypeDismissReview // 32 Dismiss Review

	CommentTypeChangeIssueRef // 33 Change issue ref

	CommentTypePRScheduledToAutoMerge   // 34 pr was scheduled to auto merge when checks succeed
	CommentTypePRUnScheduledToAutoMerge // 35 pr was un scheduled to auto merge when checks succeed

	CommentTypePin   // 36 pin Issue
	CommentTypeUnpin // 37 unpin Issue
)

var commentStrings = []string{
	"comment",
	"reopen",
	"close",
	"issue_ref",
	"commit_ref",
	"comment_ref",
	"pull_ref",
	"label",
	"milestone",
	"assignees",
	"change_title",
	"delete_branch",
	"start_tracking",
	"stop_tracking",
	"add_time_manual",
	"cancel_tracking",
	"added_deadline",
	"modified_deadline",
	"removed_deadline",
	"add_dependency",
	"remove_dependency",
	"code",
	"review",
	"lock",
	"unlock",
	"change_target_branch",
	"delete_time_manual",
	"review_request",
	"merge_pull",
	"pull_push",
	"project",
	"project_board",
	"dismiss_review",
	"change_issue_ref",
	"pull_scheduled_merge",
	"pull_cancel_scheduled_merge",
	"pin",
	"unpin",
}

func (t CommentType) String() string {
	return commentStrings[t]
}

func AsCommentType(typeName string) CommentType {
	for index, name := range commentStrings {
		if typeName == name {
			return CommentType(index)
		}
	}
	return CommentTypeUndefined
}

func (t CommentType) HasContentSupport() bool {
	switch t {
	case CommentTypeComment, CommentTypeCode, CommentTypeReview, CommentTypeDismissReview:
		return true
	}
	return false
}

func (t CommentType) HasAttachmentSupport() bool {
	switch t {
	case CommentTypeComment, CommentTypeCode, CommentTypeReview:
		return true
	}
	return false
}

// RoleDescriptor defines comment tag type
type RoleDescriptor int

// Enumerate all the role tags.
const (
	RoleDescriptorNone RoleDescriptor = iota
	RoleDescriptorPoster
	RoleDescriptorWriter
	RoleDescriptorOwner
)

// WithRole enable a specific tag on the RoleDescriptor.
func (rd RoleDescriptor) WithRole(role RoleDescriptor) RoleDescriptor {
	return rd | (1 << role)
}

func stringToRoleDescriptor(role string) RoleDescriptor {
	switch role {
	case "Poster":
		return RoleDescriptorPoster
	case "Writer":
		return RoleDescriptorWriter
	case "Owner":
		return RoleDescriptorOwner
	default:
		return RoleDescriptorNone
	}
}

// HasRole returns if a certain role is enabled on the RoleDescriptor.
func (rd RoleDescriptor) HasRole(role string) bool {
	roleDescriptor := stringToRoleDescriptor(role)
	bitValue := rd & (1 << roleDescriptor)
	return (bitValue > 0)
}

// Comment represents a comment in commit and issue page.
type Comment struct {
	ID               int64            `xorm:"pk autoincr"`
	Type             CommentType      `xorm:"INDEX"`
	PosterID         int64            `xorm:"INDEX"`
	Poster           *user_model.User `xorm:"-"`
	OriginalAuthor   string
	OriginalAuthorID int64
	IssueID          int64  `xorm:"INDEX"`
	Issue            *Issue `xorm:"-"`
	LabelID          int64
	Label            *Label   `xorm:"-"`
	AddedLabels      []*Label `xorm:"-"`
	RemovedLabels    []*Label `xorm:"-"`
	OldProjectID     int64
	ProjectID        int64
	OldProject       *project_model.Project `xorm:"-"`
	Project          *project_model.Project `xorm:"-"`
	OldMilestoneID   int64
	MilestoneID      int64
	OldMilestone     *Milestone `xorm:"-"`
	Milestone        *Milestone `xorm:"-"`
	TimeID           int64
	Time             *TrackedTime `xorm:"-"`
	AssigneeID       int64
	RemovedAssignee  bool
	Assignee         *user_model.User   `xorm:"-"`
	AssigneeTeamID   int64              `xorm:"NOT NULL DEFAULT 0"`
	AssigneeTeam     *organization.Team `xorm:"-"`
	ResolveDoerID    int64
	ResolveDoer      *user_model.User `xorm:"-"`
	OldTitle         string
	NewTitle         string
	OldRef           string
	NewRef           string
	DependentIssueID int64
	DependentIssue   *Issue `xorm:"-"`

	CommitID        int64
	Line            int64 // - previous line / + proposed line
	TreePath        string
	Content         string `xorm:"LONGTEXT"`
	RenderedContent string `xorm:"-"`

	// Path represents the 4 lines of code cemented by this comment
	Patch       string `xorm:"-"`
	PatchQuoted string `xorm:"LONGTEXT patch"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	// Reference issue in commit message
	CommitSHA string `xorm:"VARCHAR(40)"`

	Attachments []*repo_model.Attachment `xorm:"-"`
	Reactions   ReactionList             `xorm:"-"`

	// For view issue page.
	ShowRole RoleDescriptor `xorm:"-"`

	Review      *Review `xorm:"-"`
	ReviewID    int64   `xorm:"index"`
	Invalidated bool

	// Reference an issue or pull from another comment, issue or PR
	// All information is about the origin of the reference
	RefRepoID    int64                 `xorm:"index"` // Repo where the referencing
	RefIssueID   int64                 `xorm:"index"`
	RefCommentID int64                 `xorm:"index"`    // 0 if origin is Issue title or content (or PR's)
	RefAction    references.XRefAction `xorm:"SMALLINT"` // What happens if RefIssueID resolves
	RefIsPull    bool

	RefRepo    *repo_model.Repository `xorm:"-"`
	RefIssue   *Issue                 `xorm:"-"`
	RefComment *Comment               `xorm:"-"`

	Commits     []*git_model.SignCommitWithStatuses `xorm:"-"`
	OldCommit   string                              `xorm:"-"`
	NewCommit   string                              `xorm:"-"`
	CommitsNum  int64                               `xorm:"-"`
	IsForcePush bool                                `xorm:"-"`
}

func init() {
	db.RegisterModel(new(Comment))
}

// PushActionContent is content of push pull comment
type PushActionContent struct {
	IsForcePush bool     `json:"is_force_push"`
	CommitIDs   []string `json:"commit_ids"`
}

// LoadIssue loads the issue reference for the comment
func (c *Comment) LoadIssue(ctx context.Context) (err error) {
	if c.Issue != nil {
		return nil
	}
	c.Issue, err = GetIssueByID(ctx, c.IssueID)
	return err
}

// BeforeInsert will be invoked by XORM before inserting a record
func (c *Comment) BeforeInsert() {
	c.PatchQuoted = c.Patch
	if !utf8.ValidString(c.Patch) {
		c.PatchQuoted = strconv.Quote(c.Patch)
	}
}

// BeforeUpdate will be invoked by XORM before updating a record
func (c *Comment) BeforeUpdate() {
	c.PatchQuoted = c.Patch
	if !utf8.ValidString(c.Patch) {
		c.PatchQuoted = strconv.Quote(c.Patch)
	}
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (c *Comment) AfterLoad(session *xorm.Session) {
	c.Patch = c.PatchQuoted
	if len(c.PatchQuoted) > 0 && c.PatchQuoted[0] == '"' {
		unquoted, err := strconv.Unquote(c.PatchQuoted)
		if err == nil {
			c.Patch = unquoted
		}
	}
}

// LoadPoster loads comment poster
func (c *Comment) LoadPoster(ctx context.Context) (err error) {
	if c.PosterID <= 0 || c.Poster != nil {
		return nil
	}

	c.Poster, err = user_model.GetPossibleUserByID(ctx, c.PosterID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			c.PosterID = -1
			c.Poster = user_model.NewGhostUser()
		} else {
			log.Error("getUserByID[%d]: %v", c.ID, err)
		}
	}
	return err
}

// AfterDelete is invoked from XORM after the object is deleted.
func (c *Comment) AfterDelete() {
	if c.ID <= 0 {
		return
	}

	_, err := repo_model.DeleteAttachmentsByComment(c.ID, true)
	if err != nil {
		log.Info("Could not delete files for comment %d on issue #%d: %s", c.ID, c.IssueID, err)
	}
}

// HTMLURL formats a URL-string to the issue-comment
func (c *Comment) HTMLURL() string {
	err := c.LoadIssue(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.LoadRepo(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return c.Issue.HTMLURL() + c.hashLink()
}

// Link formats a relative URL-string to the issue-comment
func (c *Comment) Link() string {
	err := c.LoadIssue(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.LoadRepo(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return c.Issue.Link() + c.hashLink()
}

func (c *Comment) hashLink() string {
	if c.Type == CommentTypeCode {
		if c.ReviewID == 0 {
			return "/files#" + c.HashTag()
		}
		if c.Review == nil {
			if err := c.LoadReview(); err != nil {
				log.Warn("LoadReview(%d): %v", c.ReviewID, err)
				return "/files#" + c.HashTag()
			}
		}
		if c.Review.Type <= ReviewTypePending {
			return "/files#" + c.HashTag()
		}
	}
	return "#" + c.HashTag()
}

// APIURL formats a API-string to the issue-comment
func (c *Comment) APIURL() string {
	err := c.LoadIssue(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.LoadRepo(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}

	return fmt.Sprintf("%s/issues/comments/%d", c.Issue.Repo.APIURL(), c.ID)
}

// IssueURL formats a URL-string to the issue
func (c *Comment) IssueURL() string {
	err := c.LoadIssue(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}

	if c.Issue.IsPull {
		return ""
	}

	err = c.Issue.LoadRepo(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return c.Issue.HTMLURL()
}

// PRURL formats a URL-string to the pull-request
func (c *Comment) PRURL() string {
	err := c.LoadIssue(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}

	err = c.Issue.LoadRepo(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}

	if !c.Issue.IsPull {
		return ""
	}
	return c.Issue.HTMLURL()
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
	return fmt.Sprintf("event-%d", c.ID)
}

// LoadLabel if comment.Type is CommentTypeLabel, then load Label
func (c *Comment) LoadLabel() error {
	var label Label
	has, err := db.GetEngine(db.DefaultContext).ID(c.LabelID).Get(&label)
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

// LoadProject if comment.Type is CommentTypeProject, then load project.
func (c *Comment) LoadProject() error {
	if c.OldProjectID > 0 {
		var oldProject project_model.Project
		has, err := db.GetEngine(db.DefaultContext).ID(c.OldProjectID).Get(&oldProject)
		if err != nil {
			return err
		} else if has {
			c.OldProject = &oldProject
		}
	}

	if c.ProjectID > 0 {
		var project project_model.Project
		has, err := db.GetEngine(db.DefaultContext).ID(c.ProjectID).Get(&project)
		if err != nil {
			return err
		} else if has {
			c.Project = &project
		}
	}

	return nil
}

// LoadMilestone if comment.Type is CommentTypeMilestone, then load milestone
func (c *Comment) LoadMilestone(ctx context.Context) error {
	if c.OldMilestoneID > 0 {
		var oldMilestone Milestone
		has, err := db.GetEngine(ctx).ID(c.OldMilestoneID).Get(&oldMilestone)
		if err != nil {
			return err
		} else if has {
			c.OldMilestone = &oldMilestone
		}
	}

	if c.MilestoneID > 0 {
		var milestone Milestone
		has, err := db.GetEngine(ctx).ID(c.MilestoneID).Get(&milestone)
		if err != nil {
			return err
		} else if has {
			c.Milestone = &milestone
		}
	}
	return nil
}

// LoadAttachments loads attachments (it never returns error, the error during `GetAttachmentsByCommentIDCtx` is ignored)
func (c *Comment) LoadAttachments(ctx context.Context) error {
	if len(c.Attachments) > 0 {
		return nil
	}

	var err error
	c.Attachments, err = repo_model.GetAttachmentsByCommentID(ctx, c.ID)
	if err != nil {
		log.Error("getAttachmentsByCommentID[%d]: %v", c.ID, err)
	}
	return nil
}

// UpdateAttachments update attachments by UUIDs for the comment
func (c *Comment) UpdateAttachments(uuids []string) error {
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
		attachments[i].IssueID = c.IssueID
		attachments[i].CommentID = c.ID
		if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
		}
	}
	return committer.Commit()
}

// LoadAssigneeUserAndTeam if comment.Type is CommentTypeAssignees, then load assignees
func (c *Comment) LoadAssigneeUserAndTeam() error {
	var err error

	if c.AssigneeID > 0 && c.Assignee == nil {
		c.Assignee, err = user_model.GetUserByID(db.DefaultContext, c.AssigneeID)
		if err != nil {
			if !user_model.IsErrUserNotExist(err) {
				return err
			}
			c.Assignee = user_model.NewGhostUser()
		}
	} else if c.AssigneeTeamID > 0 && c.AssigneeTeam == nil {
		if err = c.LoadIssue(db.DefaultContext); err != nil {
			return err
		}

		if err = c.Issue.LoadRepo(db.DefaultContext); err != nil {
			return err
		}

		if err = c.Issue.Repo.LoadOwner(db.DefaultContext); err != nil {
			return err
		}

		if c.Issue.Repo.Owner.IsOrganization() {
			c.AssigneeTeam, err = organization.GetTeamByID(db.DefaultContext, c.AssigneeTeamID)
			if err != nil && !organization.IsErrTeamNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// LoadResolveDoer if comment.Type is CommentTypeCode and ResolveDoerID not zero, then load resolveDoer
func (c *Comment) LoadResolveDoer() (err error) {
	if c.ResolveDoerID == 0 || c.Type != CommentTypeCode {
		return nil
	}
	c.ResolveDoer, err = user_model.GetUserByID(db.DefaultContext, c.ResolveDoerID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			c.ResolveDoer = user_model.NewGhostUser()
			err = nil
		}
	}
	return err
}

// IsResolved check if an code comment is resolved
func (c *Comment) IsResolved() bool {
	return c.ResolveDoerID != 0 && c.Type == CommentTypeCode
}

// LoadDepIssueDetails loads Dependent Issue Details
func (c *Comment) LoadDepIssueDetails() (err error) {
	if c.DependentIssueID <= 0 || c.DependentIssue != nil {
		return nil
	}
	c.DependentIssue, err = GetIssueByID(db.DefaultContext, c.DependentIssueID)
	return err
}

// LoadTime loads the associated time for a CommentTypeAddTimeManual
func (c *Comment) LoadTime() error {
	if c.Time != nil || c.TimeID == 0 {
		return nil
	}
	var err error
	c.Time, err = GetTrackedTimeByID(c.TimeID)
	return err
}

func (c *Comment) loadReactions(ctx context.Context, repo *repo_model.Repository) (err error) {
	if c.Reactions != nil {
		return nil
	}
	c.Reactions, _, err = FindReactions(ctx, FindReactionsOptions{
		IssueID:   c.IssueID,
		CommentID: c.ID,
	})
	if err != nil {
		return err
	}
	// Load reaction user data
	if _, err := c.Reactions.LoadUsers(ctx, repo); err != nil {
		return err
	}
	return nil
}

// LoadReactions loads comment reactions
func (c *Comment) LoadReactions(repo *repo_model.Repository) error {
	return c.loadReactions(db.DefaultContext, repo)
}

func (c *Comment) loadReview(ctx context.Context) (err error) {
	if c.Review == nil {
		if c.Review, err = GetReviewByID(ctx, c.ReviewID); err != nil {
			return err
		}
	}
	c.Review.Issue = c.Issue
	return nil
}

// LoadReview loads the associated review
func (c *Comment) LoadReview() error {
	return c.loadReview(db.DefaultContext)
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

// CodeCommentLink returns the url to a comment in code
func (c *Comment) CodeCommentLink() string {
	err := c.LoadIssue(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.LoadRepo(db.DefaultContext)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return fmt.Sprintf("%s/files#%s", c.Issue.Link(), c.HashTag())
}

// LoadPushCommits Load push commits
func (c *Comment) LoadPushCommits(ctx context.Context) (err error) {
	if c.Content == "" || c.Commits != nil || c.Type != CommentTypePullRequestPush {
		return nil
	}

	var data PushActionContent

	err = json.Unmarshal([]byte(c.Content), &data)
	if err != nil {
		return
	}

	c.IsForcePush = data.IsForcePush

	if c.IsForcePush {
		if len(data.CommitIDs) != 2 {
			return nil
		}
		c.OldCommit = data.CommitIDs[0]
		c.NewCommit = data.CommitIDs[1]
	} else {
		repoPath := c.Issue.Repo.RepoPath()
		gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repoPath)
		if err != nil {
			return err
		}
		defer closer.Close()

		c.Commits = git_model.ConvertFromGitCommit(ctx, gitRepo.GetCommitsFromIDs(data.CommitIDs), c.Issue.Repo)
		c.CommitsNum = int64(len(c.Commits))
	}

	return err
}

// CreateComment creates comment with context
func CreateComment(ctx context.Context, opts *CreateCommentOptions) (_ *Comment, err error) {
	e := db.GetEngine(ctx)
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
		OldProjectID:     opts.OldProjectID,
		ProjectID:        opts.ProjectID,
		TimeID:           opts.TimeID,
		RemovedAssignee:  opts.RemovedAssignee,
		AssigneeID:       opts.AssigneeID,
		AssigneeTeamID:   opts.AssigneeTeamID,
		CommitID:         opts.CommitID,
		CommitSHA:        opts.CommitSHA,
		Line:             opts.LineNum,
		Content:          opts.Content,
		OldTitle:         opts.OldTitle,
		NewTitle:         opts.NewTitle,
		OldRef:           opts.OldRef,
		NewRef:           opts.NewRef,
		DependentIssueID: opts.DependentIssueID,
		TreePath:         opts.TreePath,
		ReviewID:         opts.ReviewID,
		Patch:            opts.Patch,
		RefRepoID:        opts.RefRepoID,
		RefIssueID:       opts.RefIssueID,
		RefCommentID:     opts.RefCommentID,
		RefAction:        opts.RefAction,
		RefIsPull:        opts.RefIsPull,
		IsForcePush:      opts.IsForcePush,
		Invalidated:      opts.Invalidated,
	}
	if _, err = e.Insert(comment); err != nil {
		return nil, err
	}

	if err = opts.Repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	if err = updateCommentInfos(ctx, opts, comment); err != nil {
		return nil, err
	}

	if err = comment.AddCrossReferences(ctx, opts.Doer, false); err != nil {
		return nil, err
	}

	return comment, nil
}

func updateCommentInfos(ctx context.Context, opts *CreateCommentOptions, comment *Comment) (err error) {
	// Check comment type.
	switch opts.Type {
	case CommentTypeCode:
		if comment.ReviewID != 0 {
			if comment.Review == nil {
				if err := comment.loadReview(ctx); err != nil {
					return err
				}
			}
			if comment.Review.Type <= ReviewTypePending {
				return nil
			}
		}
		fallthrough
	case CommentTypeComment:
		if _, err = db.Exec(ctx, "UPDATE `issue` SET num_comments=num_comments+1 WHERE id=?", opts.Issue.ID); err != nil {
			return err
		}
		fallthrough
	case CommentTypeReview:
		// Check attachments
		attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, opts.Attachments)
		if err != nil {
			return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", opts.Attachments, err)
		}

		for i := range attachments {
			attachments[i].IssueID = opts.Issue.ID
			attachments[i].CommentID = comment.ID
			// No assign value could be 0, so ignore AllCols().
			if _, err = db.GetEngine(ctx).ID(attachments[i].ID).Update(attachments[i]); err != nil {
				return fmt.Errorf("update attachment [%d]: %w", attachments[i].ID, err)
			}
		}

		comment.Attachments = attachments
	case CommentTypeReopen, CommentTypeClose:
		if err = repo_model.UpdateRepoIssueNumbers(ctx, opts.Issue.RepoID, opts.Issue.IsPull, true); err != nil {
			return err
		}
	}
	// update the issue's updated_unix column
	return UpdateIssueCols(ctx, opts.Issue, "updated_unix")
}

func createDeadlineComment(ctx context.Context, doer *user_model.User, issue *Issue, newDeadlineUnix timeutil.TimeStamp) (*Comment, error) {
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

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	opts := &CreateCommentOptions{
		Type:    commentType,
		Doer:    doer,
		Repo:    issue.Repo,
		Issue:   issue,
		Content: content,
	}
	comment, err := CreateComment(ctx, opts)
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// Creates issue dependency comment
func createIssueDependencyComment(ctx context.Context, doer *user_model.User, issue, dependentIssue *Issue, add bool) (err error) {
	cType := CommentTypeAddDependency
	if !add {
		cType = CommentTypeRemoveDependency
	}
	if err = issue.LoadRepo(ctx); err != nil {
		return
	}

	// Make two comments, one in each issue
	opts := &CreateCommentOptions{
		Type:             cType,
		Doer:             doer,
		Repo:             issue.Repo,
		Issue:            issue,
		DependentIssueID: dependentIssue.ID,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return
	}

	opts = &CreateCommentOptions{
		Type:             cType,
		Doer:             doer,
		Repo:             issue.Repo,
		Issue:            dependentIssue,
		DependentIssueID: issue.ID,
	}
	_, err = CreateComment(ctx, opts)
	return err
}

// CreateCommentOptions defines options for creating comment
type CreateCommentOptions struct {
	Type  CommentType
	Doer  *user_model.User
	Repo  *repo_model.Repository
	Issue *Issue
	Label *Label

	DependentIssueID int64
	OldMilestoneID   int64
	MilestoneID      int64
	OldProjectID     int64
	ProjectID        int64
	TimeID           int64
	AssigneeID       int64
	AssigneeTeamID   int64
	RemovedAssignee  bool
	OldTitle         string
	NewTitle         string
	OldRef           string
	NewRef           string
	CommitID         int64
	CommitSHA        string
	Patch            string
	LineNum          int64
	TreePath         string
	ReviewID         int64
	Content          string
	Attachments      []string // UUIDs of attachments
	RefRepoID        int64
	RefIssueID       int64
	RefCommentID     int64
	RefAction        references.XRefAction
	RefIsPull        bool
	IsForcePush      bool
	Invalidated      bool
}

// GetCommentByID returns the comment by given ID.
func GetCommentByID(ctx context.Context, id int64) (*Comment, error) {
	c := new(Comment)
	has, err := db.GetEngine(ctx).ID(id).Get(c)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{id, 0}
	}
	return c, nil
}

// FindCommentsOptions describes the conditions to Find comments
type FindCommentsOptions struct {
	db.ListOptions
	RepoID      int64
	IssueID     int64
	ReviewID    int64
	Since       int64
	Before      int64
	Line        int64
	TreePath    string
	Type        CommentType
	IssueIDs    []int64
	Invalidated util.OptionalBool
	IsPull      util.OptionalBool
}

// ToConds implements FindOptions interface
func (opts *FindCommentsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"issue.repo_id": opts.RepoID})
	}
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"comment.issue_id": opts.IssueID})
	} else if len(opts.IssueIDs) > 0 {
		cond = cond.And(builder.In("comment.issue_id", opts.IssueIDs))
	}
	if opts.ReviewID > 0 {
		cond = cond.And(builder.Eq{"comment.review_id": opts.ReviewID})
	}
	if opts.Since > 0 {
		cond = cond.And(builder.Gte{"comment.updated_unix": opts.Since})
	}
	if opts.Before > 0 {
		cond = cond.And(builder.Lte{"comment.updated_unix": opts.Before})
	}
	if opts.Type != CommentTypeUndefined {
		cond = cond.And(builder.Eq{"comment.type": opts.Type})
	}
	if opts.Line != 0 {
		cond = cond.And(builder.Eq{"comment.line": opts.Line})
	}
	if len(opts.TreePath) > 0 {
		cond = cond.And(builder.Eq{"comment.tree_path": opts.TreePath})
	}
	if !opts.Invalidated.IsNone() {
		cond = cond.And(builder.Eq{"comment.invalidated": opts.Invalidated.IsTrue()})
	}
	if opts.IsPull != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"issue.is_pull": opts.IsPull.IsTrue()})
	}
	return cond
}

// FindComments returns all comments according options
func FindComments(ctx context.Context, opts *FindCommentsOptions) (CommentList, error) {
	comments := make([]*Comment, 0, 10)
	sess := db.GetEngine(ctx).Where(opts.ToConds())
	if opts.RepoID > 0 || opts.IsPull != util.OptionalBoolNone {
		sess.Join("INNER", "issue", "issue.id = comment.issue_id")
	}

	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, opts)
	}

	// WARNING: If you change this order you will need to fix createCodeComment

	return comments, sess.
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments)
}

// CountComments count all comments according options by ignoring pagination
func CountComments(opts *FindCommentsOptions) (int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where(opts.ToConds())
	if opts.RepoID > 0 {
		sess.Join("INNER", "issue", "issue.id = comment.issue_id")
	}
	return sess.Count(&Comment{})
}

// UpdateCommentInvalidate updates comment invalidated column
func UpdateCommentInvalidate(ctx context.Context, c *Comment) error {
	_, err := db.GetEngine(ctx).ID(c.ID).Cols("invalidated").Update(c)
	return err
}

// UpdateComment updates information of comment.
func UpdateComment(c *Comment, doer *user_model.User) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	if _, err := sess.ID(c.ID).AllCols().Update(c); err != nil {
		return err
	}
	if err := c.LoadIssue(ctx); err != nil {
		return err
	}
	if err := c.AddCrossReferences(ctx, doer, true); err != nil {
		return err
	}
	if err := committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(ctx context.Context, comment *Comment) error {
	e := db.GetEngine(ctx)
	if _, err := e.ID(comment.ID).NoAutoCondition().Delete(comment); err != nil {
		return err
	}

	if _, err := db.DeleteByBean(ctx, &ContentHistory{
		CommentID: comment.ID,
	}); err != nil {
		return err
	}

	if comment.Type == CommentTypeComment {
		if _, err := e.ID(comment.IssueID).Decr("num_comments").Update(new(Issue)); err != nil {
			return err
		}
	}
	if _, err := e.Table("action").
		Where("comment_id = ?", comment.ID).
		Update(map[string]any{
			"is_deleted": true,
		}); err != nil {
		return err
	}

	if err := comment.neuterCrossReferences(ctx); err != nil {
		return err
	}

	return DeleteReaction(ctx, &ReactionOptions{CommentID: comment.ID})
}

// UpdateCommentsMigrationsByType updates comments' migrations information via given git service type and original id and poster id
func UpdateCommentsMigrationsByType(tp structs.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Table("comment").
		Where(builder.In("issue_id",
			builder.Select("issue.id").
				From("issue").
				InnerJoin("repository", "issue.repo_id = repository.id").
				Where(builder.Eq{
					"repository.original_service_type": tp,
				}),
		)).
		And("comment.original_author_id = ?", originalAuthorID).
		Update(map[string]any{
			"poster_id":          posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// CreateAutoMergeComment is a internal function, only use it for CommentTypePRScheduledToAutoMerge and CommentTypePRUnScheduledToAutoMerge CommentTypes
func CreateAutoMergeComment(ctx context.Context, typ CommentType, pr *PullRequest, doer *user_model.User) (comment *Comment, err error) {
	if typ != CommentTypePRScheduledToAutoMerge && typ != CommentTypePRUnScheduledToAutoMerge {
		return nil, fmt.Errorf("comment type %d cannot be used to create an auto merge comment", typ)
	}
	if err = pr.LoadIssue(ctx); err != nil {
		return
	}

	if err = pr.LoadBaseRepo(ctx); err != nil {
		return
	}

	comment, err = CreateComment(ctx, &CreateCommentOptions{
		Type:  typ,
		Doer:  doer,
		Repo:  pr.BaseRepo,
		Issue: pr.Issue,
	})
	return comment, err
}

// RemapExternalUser ExternalUserRemappable interface
func (c *Comment) RemapExternalUser(externalName string, externalID, userID int64) error {
	c.OriginalAuthor = externalName
	c.OriginalAuthorID = externalID
	c.PosterID = userID
	return nil
}

// GetUserID ExternalUserRemappable interface
func (c *Comment) GetUserID() int64 { return c.PosterID }

// GetExternalName ExternalUserRemappable interface
func (c *Comment) GetExternalName() string { return c.OriginalAuthor }

// GetExternalID ExternalUserRemappable interface
func (c *Comment) GetExternalID() int64 { return c.OriginalAuthorID }

// CountCommentTypeLabelWithEmptyLabel count label comments with empty label
func CountCommentTypeLabelWithEmptyLabel(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where(builder.Eq{"type": CommentTypeLabel, "label_id": 0}).Count(new(Comment))
}

// FixCommentTypeLabelWithEmptyLabel count label comments with empty label
func FixCommentTypeLabelWithEmptyLabel(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where(builder.Eq{"type": CommentTypeLabel, "label_id": 0}).Delete(new(Comment))
}

// CountCommentTypeLabelWithOutsideLabels count label comments with outside label
func CountCommentTypeLabelWithOutsideLabels(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where("comment.type = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id))", CommentTypeLabel).
		Table("comment").
		Join("inner", "label", "label.id = comment.label_id").
		Join("inner", "issue", "issue.id = comment.issue_id ").
		Join("inner", "repository", "issue.repo_id = repository.id").
		Count()
}

// FixCommentTypeLabelWithOutsideLabels count label comments with outside label
func FixCommentTypeLabelWithOutsideLabels(ctx context.Context) (int64, error) {
	res, err := db.GetEngine(ctx).Exec(`DELETE FROM comment WHERE comment.id IN (
		SELECT il_too.id FROM (
			SELECT com.id
				FROM comment AS com
					INNER JOIN label ON com.label_id = label.id
					INNER JOIN issue on issue.id = com.issue_id
					INNER JOIN repository ON issue.repo_id = repository.id
				WHERE
					com.type = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != repository.owner_id))
	) AS il_too)`, CommentTypeLabel)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// HasOriginalAuthor returns if a comment was migrated and has an original author.
func (c *Comment) HasOriginalAuthor() bool {
	return c.OriginalAuthor != "" && c.OriginalAuthorID != 0
}
