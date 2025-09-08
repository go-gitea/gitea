// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"errors"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
)

// ErrInvalidReceiveHook FIXME
var ErrInvalidReceiveHook = errors.New("Invalid JSON payload received over webhook")

// Hook a hook is a web hook when one repository changed
type Hook struct {
	// The unique identifier of the webhook
	ID int64 `json:"id"`
	// The type of the webhook (e.g., gitea, slack, discord)
	Type string `json:"type"`
	// Branch filter pattern to determine which branches trigger the webhook
	BranchFilter string `json:"branch_filter"`
	// The URL of the webhook endpoint (hidden in JSON)
	URL string `json:"-"`
	// Configuration settings for the webhook
	Config map[string]string `json:"config"`
	// List of events that trigger this webhook
	Events []string `json:"events"`
	// Authorization header to include in webhook requests
	AuthorizationHeader string `json:"authorization_header"`
	// Whether the webhook is active and will be triggered
	Active bool `json:"active"`
	// swagger:strfmt date-time
	// The date and time when the webhook was last updated
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	// The date and time when the webhook was created
	Created time.Time `json:"created_at"`
}

// HookList represents a list of API hook.
type HookList []*Hook

// CreateHookOptionConfig has all config options in it
// required are "content_type" and "url" Required
type CreateHookOptionConfig map[string]string

// CreateHookOption options when create a hook
type CreateHookOption struct {
	// required: true
	// enum: dingtalk,discord,gitea,gogs,msteams,slack,telegram,feishu,wechatwork,packagist
	// The type of the webhook to create
	Type string `json:"type" binding:"Required"`
	// required: true
	// Configuration settings for the webhook
	Config CreateHookOptionConfig `json:"config" binding:"Required"`
	// List of events that will trigger this webhook
	Events []string `json:"events"`
	// Branch filter pattern to determine which branches trigger the webhook
	BranchFilter string `json:"branch_filter" binding:"GlobPattern"`
	// Authorization header to include in webhook requests
	AuthorizationHeader string `json:"authorization_header"`
	// default: false
	// Whether the webhook should be active upon creation
	Active bool `json:"active"`
}

// EditHookOption options when modify one hook
type EditHookOption struct {
	// Configuration settings for the webhook
	Config map[string]string `json:"config"`
	// List of events that trigger this webhook
	Events []string `json:"events"`
	// Branch filter pattern to determine which branches trigger the webhook
	BranchFilter string `json:"branch_filter" binding:"GlobPattern"`
	// Authorization header to include in webhook requests
	AuthorizationHeader string `json:"authorization_header"`
	// Whether the webhook is active and will be triggered
	Active *bool `json:"active"`
}

// Payloader payload is some part of one hook
type Payloader interface {
	JSONPayload() ([]byte, error)
}

// PayloadUser represents the author or committer of a commit
type PayloadUser struct {
	// Full name of the commit author
	Name string `json:"name"`
	// swagger:strfmt email
	Email string `json:"email"`
	// username of the user
	UserName string `json:"username"`
}

// FIXME: consider using same format as API when commits API are added.
//        applies to PayloadCommit and PayloadCommitVerification

// PayloadCommit represents a commit
type PayloadCommit struct {
	// sha1 hash of the commit
	ID string `json:"id"`
	// The commit message
	Message string `json:"message"`
	// The URL to view this commit
	URL string `json:"url"`
	// The author of the commit
	Author *PayloadUser `json:"author"`
	// The committer of the commit
	Committer *PayloadUser `json:"committer"`
	// GPG verification information for the commit
	Verification *PayloadCommitVerification `json:"verification"`
	// swagger:strfmt date-time
	// The timestamp when the commit was made
	Timestamp time.Time `json:"timestamp"`
	// List of files added in this commit
	Added []string `json:"added"`
	// List of files removed in this commit
	Removed []string `json:"removed"`
	// List of files modified in this commit
	Modified []string `json:"modified"`
}

// PayloadCommitVerification represents the GPG verification of a commit
type PayloadCommitVerification struct {
	// Whether the commit signature is verified
	Verified bool `json:"verified"`
	// The reason for the verification status
	Reason string `json:"reason"`
	// The GPG signature of the commit
	Signature string `json:"signature"`
	// The user who signed the commit
	Signer *PayloadUser `json:"signer"`
	// The signed payload content
	Payload string `json:"payload"`
}

var (
	_ Payloader = &CreatePayload{}
	_ Payloader = &DeletePayload{}
	_ Payloader = &ForkPayload{}
	_ Payloader = &PushPayload{}
	_ Payloader = &IssuePayload{}
	_ Payloader = &IssueCommentPayload{}
	_ Payloader = &PullRequestPayload{}
	_ Payloader = &RepositoryPayload{}
	_ Payloader = &ReleasePayload{}
	_ Payloader = &PackagePayload{}
)

// CreatePayload represents a payload information of create event.
type CreatePayload struct {
	// The SHA hash of the created reference
	Sha string `json:"sha"`
	// The full name of the created reference
	Ref string `json:"ref"`
	// The type of reference created (branch or tag)
	RefType string `json:"ref_type"`
	// The repository where the reference was created
	Repo *Repository `json:"repository"`
	// The user who created the reference
	Sender *User `json:"sender"`
}

// JSONPayload return payload information
func (p *CreatePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ParseCreateHook parses create event hook content.
func ParseCreateHook(raw []byte) (*CreatePayload, error) {
	hook := new(CreatePayload)
	if err := json.Unmarshal(raw, hook); err != nil {
		return nil, err
	}

	// it is possible the JSON was parsed, however,
	// was not from Gogs (maybe was from Bitbucket)
	// So we'll check to be sure certain key fields
	// were populated
	switch {
	case hook.Repo == nil:
		return nil, ErrInvalidReceiveHook
	case len(hook.Ref) == 0:
		return nil, ErrInvalidReceiveHook
	}
	return hook, nil
}

// PusherType define the type to push
type PusherType string

// describe all the PusherTypes
const (
	PusherTypeUser PusherType = "user"
)

// DeletePayload represents delete payload
type DeletePayload struct {
	// The name of the deleted reference
	Ref string `json:"ref"`
	// The type of reference deleted (branch or tag)
	RefType string `json:"ref_type"`
	// The type of entity that performed the deletion
	PusherType PusherType `json:"pusher_type"`
	// The repository where the reference was deleted
	Repo *Repository `json:"repository"`
	// The user who deleted the reference
	Sender *User `json:"sender"`
}

// JSONPayload implements Payload
func (p *DeletePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ForkPayload represents fork payload
type ForkPayload struct {
	// The forked repository (the new fork)
	Forkee *Repository `json:"forkee"`
	// The original repository that was forked
	Repo *Repository `json:"repository"`
	// The user who created the fork
	Sender *User `json:"sender"`
}

// JSONPayload implements Payload
func (p *ForkPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// HookIssueCommentAction defines hook issue comment action
type HookIssueCommentAction string

// all issue comment actions
const (
	HookIssueCommentCreated HookIssueCommentAction = "created"
	HookIssueCommentEdited  HookIssueCommentAction = "edited"
	HookIssueCommentDeleted HookIssueCommentAction = "deleted"
)

// IssueCommentPayload represents a payload information of issue comment event.
type IssueCommentPayload struct {
	// The action performed on the comment (created, edited, deleted)
	Action HookIssueCommentAction `json:"action"`
	// The issue that the comment belongs to
	Issue *Issue `json:"issue"`
	// The pull request if the comment is on a pull request
	PullRequest *PullRequest `json:"pull_request,omitempty"`
	// The comment that was acted upon
	Comment *Comment `json:"comment"`
	// Changes made to the comment (for edit actions)
	Changes *ChangesPayload `json:"changes,omitempty"`
	// The repository containing the issue/pull request
	Repository *Repository `json:"repository"`
	// The user who performed the action
	Sender *User `json:"sender"`
	// Whether this comment is on a pull request
	IsPull bool `json:"is_pull"`
}

// JSONPayload implements Payload
func (p *IssueCommentPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// HookReleaseAction defines hook release action type
type HookReleaseAction string

// all release actions
const (
	HookReleasePublished HookReleaseAction = "published"
	HookReleaseUpdated   HookReleaseAction = "updated"
	HookReleaseDeleted   HookReleaseAction = "deleted"
)

// ReleasePayload represents a payload information of release event.
type ReleasePayload struct {
	// The action performed on the release (published, updated, deleted)
	Action HookReleaseAction `json:"action"`
	// The release that was acted upon
	Release *Release `json:"release"`
	// The repository containing the release
	Repository *Repository `json:"repository"`
	// The user who performed the action
	Sender *User `json:"sender"`
}

// JSONPayload implements Payload
func (p *ReleasePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// PushPayload represents a payload information of push event.
type PushPayload struct {
	// The full name of the pushed reference
	Ref string `json:"ref"`
	// The SHA of the most recent commit before the push
	Before string `json:"before"`
	// The SHA of the most recent commit after the push
	After string `json:"after"`
	// URL to compare the changes in this push
	CompareURL string `json:"compare_url"`
	// List of commits included in the push
	Commits []*PayloadCommit `json:"commits"`
	// Total number of commits in the push
	TotalCommits int `json:"total_commits"`
	// The most recent commit in the push
	HeadCommit *PayloadCommit `json:"head_commit"`
	// The repository that was pushed to
	Repo *Repository `json:"repository"`
	// The user who performed the push
	Pusher *User `json:"pusher"`
	// The user who triggered the webhook
	Sender *User `json:"sender"`
}

// JSONPayload FIXME
func (p *PushPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ParsePushHook parses push event hook content.
func ParsePushHook(raw []byte) (*PushPayload, error) {
	hook := new(PushPayload)
	if err := json.Unmarshal(raw, hook); err != nil {
		return nil, err
	}

	switch {
	case hook.Repo == nil:
		return nil, ErrInvalidReceiveHook
	case len(hook.Ref) == 0:
		return nil, ErrInvalidReceiveHook
	}
	return hook, nil
}

// Branch returns branch name from a payload
func (p *PushPayload) Branch() string {
	return strings.ReplaceAll(p.Ref, "refs/heads/", "")
}

// HookIssueAction FIXME
type HookIssueAction string

const (
	// HookIssueOpened opened
	HookIssueOpened HookIssueAction = "opened"
	// HookIssueClosed closed
	HookIssueClosed HookIssueAction = "closed"
	// HookIssueReOpened reopened
	HookIssueReOpened HookIssueAction = "reopened"
	// HookIssueEdited edited
	HookIssueEdited HookIssueAction = "edited"
	// HookIssueDeleted is an issue action for deleting an issue
	HookIssueDeleted HookIssueAction = "deleted"
	// HookIssueAssigned assigned
	HookIssueAssigned HookIssueAction = "assigned"
	// HookIssueUnassigned unassigned
	HookIssueUnassigned HookIssueAction = "unassigned"
	// HookIssueLabelUpdated label_updated
	HookIssueLabelUpdated HookIssueAction = "label_updated"
	// HookIssueLabelCleared label_cleared
	HookIssueLabelCleared HookIssueAction = "label_cleared"
	// HookIssueSynchronized synchronized
	HookIssueSynchronized HookIssueAction = "synchronized"
	// HookIssueMilestoned is an issue action for when a milestone is set on an issue.
	HookIssueMilestoned HookIssueAction = "milestoned"
	// HookIssueDemilestoned is an issue action for when a milestone is cleared on an issue.
	HookIssueDemilestoned HookIssueAction = "demilestoned"
	// HookIssueReviewed is an issue action for when a pull request is reviewed
	HookIssueReviewed HookIssueAction = "reviewed"
	// HookIssueReviewRequested is an issue action for when a reviewer is requested for a pull request.
	HookIssueReviewRequested HookIssueAction = "review_requested"
	// HookIssueReviewRequestRemoved is an issue action for removing a review request to someone on a pull request.
	HookIssueReviewRequestRemoved HookIssueAction = "review_request_removed"
)

// IssuePayload represents the payload information that is sent along with an issue event.
type IssuePayload struct {
	// The action performed on the issue
	Action HookIssueAction `json:"action"`
	// The index number of the issue
	Index int64 `json:"number"`
	// Changes made to the issue (for edit actions)
	Changes *ChangesPayload `json:"changes,omitempty"`
	// The issue that was acted upon
	Issue *Issue `json:"issue"`
	// The repository containing the issue
	Repository *Repository `json:"repository"`
	// The user who performed the action
	Sender *User `json:"sender"`
	// The commit ID related to the issue action
	CommitID string `json:"commit_id"`
}

// JSONPayload encodes the IssuePayload to JSON, with an indentation of two spaces.
func (p *IssuePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ChangesFromPayload FIXME
type ChangesFromPayload struct {
	// The previous value before the change
	From string `json:"from"`
}

// ChangesPayload represents the payload information of issue change
type ChangesPayload struct {
	// Changes made to the title
	Title *ChangesFromPayload `json:"title,omitempty"`
	// Changes made to the body/description
	Body *ChangesFromPayload `json:"body,omitempty"`
	// Changes made to the reference
	Ref *ChangesFromPayload `json:"ref,omitempty"`
}

// PullRequestPayload represents a payload information of pull request event.
type PullRequestPayload struct {
	// The action performed on the pull request
	Action HookIssueAction `json:"action"`
	// The index number of the pull request
	Index int64 `json:"number"`
	// Changes made to the pull request (for edit actions)
	Changes *ChangesPayload `json:"changes,omitempty"`
	// The pull request that was acted upon
	PullRequest *PullRequest `json:"pull_request"`
	// The reviewer that was requested (for review request actions)
	RequestedReviewer *User `json:"requested_reviewer"`
	// The repository containing the pull request
	Repository *Repository `json:"repository"`
	// The user who performed the action
	Sender *User `json:"sender"`
	// The commit ID related to the pull request action
	CommitID string `json:"commit_id"`
	// The review information (for review actions)
	Review *ReviewPayload `json:"review"`
}

// JSONPayload FIXME
func (p *PullRequestPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ReviewPayload FIXME
type ReviewPayload struct {
	// The type of review (approved, rejected, comment)
	Type string `json:"type"`
	// The content/body of the review
	Content string `json:"content"`
}

// HookWikiAction an action that happens to a wiki page
type HookWikiAction string

const (
	// HookWikiCreated created
	HookWikiCreated HookWikiAction = "created"
	// HookWikiEdited edited
	HookWikiEdited HookWikiAction = "edited"
	// HookWikiDeleted deleted
	HookWikiDeleted HookWikiAction = "deleted"
)

// WikiPayload payload for repository webhooks
type WikiPayload struct {
	// The action performed on the wiki page
	Action HookWikiAction `json:"action"`
	// The repository containing the wiki
	Repository *Repository `json:"repository"`
	// The user who performed the action
	Sender *User `json:"sender"`
	// The name of the wiki page
	Page string `json:"page"`
	// The comment/commit message for the wiki change
	Comment string `json:"comment"`
}

// JSONPayload JSON representation of the payload
func (p *WikiPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", " ")
}

// HookRepoAction an action that happens to a repo
type HookRepoAction string

const (
	// HookRepoCreated created
	HookRepoCreated HookRepoAction = "created"
	// HookRepoDeleted deleted
	HookRepoDeleted HookRepoAction = "deleted"
)

// RepositoryPayload payload for repository webhooks
type RepositoryPayload struct {
	// The action performed on the repository
	Action HookRepoAction `json:"action"`
	// The repository that was acted upon
	Repository *Repository `json:"repository"`
	// The organization that owns the repository (if applicable)
	Organization *User `json:"organization"`
	// The user who performed the action
	Sender *User `json:"sender"`
}

// JSONPayload JSON representation of the payload
func (p *RepositoryPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", " ")
}

// HookPackageAction an action that happens to a package
type HookPackageAction string

const (
	// HookPackageCreated created
	HookPackageCreated HookPackageAction = "created"
	// HookPackageDeleted deleted
	HookPackageDeleted HookPackageAction = "deleted"
)

// PackagePayload represents a package payload
type PackagePayload struct {
	// The action performed on the package
	Action HookPackageAction `json:"action"`
	// The repository associated with the package
	Repository *Repository `json:"repository"`
	// The package that was acted upon
	Package *Package `json:"package"`
	// The organization that owns the package (if applicable)
	Organization *Organization `json:"organization"`
	// The user who performed the action
	Sender *User `json:"sender"`
}

// JSONPayload implements Payload
func (p *PackagePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// WorkflowDispatchPayload represents a workflow dispatch payload
type WorkflowDispatchPayload struct {
	// The name or path of the workflow file
	Workflow string `json:"workflow"`
	// The git reference (branch, tag, or commit SHA) to run the workflow on
	Ref string `json:"ref"`
	// Input parameters for the workflow dispatch event
	Inputs map[string]any `json:"inputs"`
	// The repository containing the workflow
	Repository *Repository `json:"repository"`
	// The user who triggered the workflow dispatch
	Sender *User `json:"sender"`
}

// JSONPayload implements Payload
func (p *WorkflowDispatchPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// CommitStatusPayload represents a payload information of commit status event.
type CommitStatusPayload struct {
	// TODO: add Branches per https://docs.github.com/en/webhooks/webhook-events-and-payloads#status
	// The commit that the status is associated with
	Commit *PayloadCommit `json:"commit"`
	// The context/identifier for this status check
	Context string `json:"context"`
	// swagger:strfmt date-time
	// The date and time when the status was created
	CreatedAt time.Time `json:"created_at"`
	// A short description of the status
	Description string `json:"description"`
	// The unique identifier of the status
	ID int64 `json:"id"`
	// The repository containing the commit
	Repo *Repository `json:"repository"`
	// The user who created the status
	Sender *User `json:"sender"`
	// The SHA hash of the commit
	SHA string `json:"sha"`
	// The state of the status (pending, success, error, failure)
	State string `json:"state"`
	// The target URL to associate with this status
	TargetURL string `json:"target_url"`
	// swagger:strfmt date-time
	// The date and time when the status was last updated
	UpdatedAt *time.Time `json:"updated_at"`
}

// JSONPayload implements Payload
func (p *CommitStatusPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// WorkflowRunPayload represents a payload information of workflow run event.
type WorkflowRunPayload struct {
	// The action performed on the workflow run
	Action string `json:"action"`
	// The workflow definition
	Workflow *ActionWorkflow `json:"workflow"`
	// The workflow run that was acted upon
	WorkflowRun *ActionWorkflowRun `json:"workflow_run"`
	// The pull request associated with the workflow run (if applicable)
	PullRequest *PullRequest `json:"pull_request,omitempty"`
	// The organization that owns the repository (if applicable)
	Organization *Organization `json:"organization,omitempty"`
	// The repository containing the workflow
	Repo *Repository `json:"repository"`
	// The user who triggered the workflow run
	Sender *User `json:"sender"`
}

// JSONPayload implements Payload
func (p *WorkflowRunPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// WorkflowJobPayload represents a payload information of workflow job event.
type WorkflowJobPayload struct {
	// The action performed on the workflow job
	Action string `json:"action"`
	// The workflow job that was acted upon
	WorkflowJob *ActionWorkflowJob `json:"workflow_job"`
	// The pull request associated with the workflow job (if applicable)
	PullRequest *PullRequest `json:"pull_request,omitempty"`
	// The organization that owns the repository (if applicable)
	Organization *Organization `json:"organization,omitempty"`
	// The repository containing the workflow
	Repo *Repository `json:"repository"`
	// The user who triggered the workflow job
	Sender *User `json:"sender"`
}

// JSONPayload implements Payload
func (p *WorkflowJobPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
