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
	ID                  int64             `json:"id"`
	Type                string            `json:"type"`
	BranchFilter        string            `json:"branch_filter"`
	URL                 string            `json:"-"`
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	AuthorizationHeader string            `json:"authorization_header"`
	Active              bool              `json:"active"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
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
	Type string `json:"type" binding:"Required"`
	// required: true
	Config              CreateHookOptionConfig `json:"config" binding:"Required"`
	Events              []string               `json:"events"`
	BranchFilter        string                 `json:"branch_filter" binding:"GlobPattern"`
	AuthorizationHeader string                 `json:"authorization_header"`
	// default: false
	Active bool `json:"active"`
}

// EditHookOption options when modify one hook
type EditHookOption struct {
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	BranchFilter        string            `json:"branch_filter" binding:"GlobPattern"`
	AuthorizationHeader string            `json:"authorization_header"`
	Active              *bool             `json:"active"`
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
	Email    string `json:"email"`
	UserName string `json:"username"`
}

// FIXME: consider using same format as API when commits API are added.
//        applies to PayloadCommit and PayloadCommitVerification

// PayloadCommit represents a commit
type PayloadCommit struct {
	// sha1 hash of the commit
	ID           string                     `json:"id"`
	Message      string                     `json:"message"`
	URL          string                     `json:"url"`
	Author       *PayloadUser               `json:"author"`
	Committer    *PayloadUser               `json:"committer"`
	Verification *PayloadCommitVerification `json:"verification"`
	// swagger:strfmt date-time
	Timestamp time.Time `json:"timestamp"`
	Added     []string  `json:"added"`
	Removed   []string  `json:"removed"`
	Modified  []string  `json:"modified"`
}

// PayloadCommitVerification represents the GPG verification of a commit
type PayloadCommitVerification struct {
	Verified  bool         `json:"verified"`
	Reason    string       `json:"reason"`
	Signature string       `json:"signature"`
	Signer    *PayloadUser `json:"signer"`
	Payload   string       `json:"payload"`
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
	Sha     string      `json:"sha"`
	Ref     string      `json:"ref"`
	RefType string      `json:"ref_type"`
	Repo    *Repository `json:"repository"`
	Sender  *User       `json:"sender"`
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
	Ref        string      `json:"ref"`
	RefType    string      `json:"ref_type"`
	PusherType PusherType  `json:"pusher_type"`
	Repo       *Repository `json:"repository"`
	Sender     *User       `json:"sender"`
}

// JSONPayload implements Payload
func (p *DeletePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ForkPayload represents fork payload
type ForkPayload struct {
	Forkee *Repository `json:"forkee"`
	Repo   *Repository `json:"repository"`
	Sender *User       `json:"sender"`
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
	Action      HookIssueCommentAction `json:"action"`
	Issue       *Issue                 `json:"issue"`
	PullRequest *PullRequest           `json:"pull_request,omitempty"`
	Comment     *Comment               `json:"comment"`
	Changes     *ChangesPayload        `json:"changes,omitempty"`
	Repository  *Repository            `json:"repository"`
	Sender      *User                  `json:"sender"`
	IsPull      bool                   `json:"is_pull"`
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
	Action     HookReleaseAction `json:"action"`
	Release    *Release          `json:"release"`
	Repository *Repository       `json:"repository"`
	Sender     *User             `json:"sender"`
}

// JSONPayload implements Payload
func (p *ReleasePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// PushPayload represents a payload information of push event.
type PushPayload struct {
	Ref          string           `json:"ref"`
	Before       string           `json:"before"`
	After        string           `json:"after"`
	CompareURL   string           `json:"compare_url"`
	Commits      []*PayloadCommit `json:"commits"`
	TotalCommits int              `json:"total_commits"`
	HeadCommit   *PayloadCommit   `json:"head_commit"`
	Repo         *Repository      `json:"repository"`
	Pusher       *User            `json:"pusher"`
	Sender       *User            `json:"sender"`
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
	Action     HookIssueAction `json:"action"`
	Index      int64           `json:"number"`
	Changes    *ChangesPayload `json:"changes,omitempty"`
	Issue      *Issue          `json:"issue"`
	Repository *Repository     `json:"repository"`
	Sender     *User           `json:"sender"`
	CommitID   string          `json:"commit_id"`
}

// JSONPayload encodes the IssuePayload to JSON, with an indentation of two spaces.
func (p *IssuePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ChangesFromPayload FIXME
type ChangesFromPayload struct {
	From string `json:"from"`
}

// ChangesPayload represents the payload information of issue change
type ChangesPayload struct {
	Title *ChangesFromPayload `json:"title,omitempty"`
	Body  *ChangesFromPayload `json:"body,omitempty"`
	Ref   *ChangesFromPayload `json:"ref,omitempty"`
}

// PullRequestPayload represents a payload information of pull request event.
type PullRequestPayload struct {
	Action            HookIssueAction `json:"action"`
	Index             int64           `json:"number"`
	Changes           *ChangesPayload `json:"changes,omitempty"`
	PullRequest       *PullRequest    `json:"pull_request"`
	RequestedReviewer *User           `json:"requested_reviewer"`
	Repository        *Repository     `json:"repository"`
	Sender            *User           `json:"sender"`
	CommitID          string          `json:"commit_id"`
	Review            *ReviewPayload  `json:"review"`
}

// JSONPayload FIXME
func (p *PullRequestPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ReviewPayload FIXME
type ReviewPayload struct {
	Type    string `json:"type"`
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
	Action     HookWikiAction `json:"action"`
	Repository *Repository    `json:"repository"`
	Sender     *User          `json:"sender"`
	Page       string         `json:"page"`
	Comment    string         `json:"comment"`
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
	Action       HookRepoAction `json:"action"`
	Repository   *Repository    `json:"repository"`
	Organization *User          `json:"organization"`
	Sender       *User          `json:"sender"`
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
	Action       HookPackageAction `json:"action"`
	Repository   *Repository       `json:"repository"`
	Package      *Package          `json:"package"`
	Organization *Organization     `json:"organization"`
	Sender       *User             `json:"sender"`
}

// JSONPayload implements Payload
func (p *PackagePayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// WorkflowDispatchPayload represents a workflow dispatch payload
type WorkflowDispatchPayload struct {
	Workflow   string         `json:"workflow"`
	Ref        string         `json:"ref"`
	Inputs     map[string]any `json:"inputs"`
	Repository *Repository    `json:"repository"`
	Sender     *User          `json:"sender"`
}

// JSONPayload implements Payload
func (p *WorkflowDispatchPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// CommitStatusPayload represents a payload information of commit status event.
type CommitStatusPayload struct {
	// TODO: add Branches per https://docs.github.com/en/webhooks/webhook-events-and-payloads#status
	Commit  *PayloadCommit `json:"commit"`
	Context string         `json:"context"`
	// swagger:strfmt date-time
	CreatedAt   time.Time   `json:"created_at"`
	Description string      `json:"description"`
	ID          int64       `json:"id"`
	Repo        *Repository `json:"repository"`
	Sender      *User       `json:"sender"`
	SHA         string      `json:"sha"`
	State       string      `json:"state"`
	TargetURL   string      `json:"target_url"`
	// swagger:strfmt date-time
	UpdatedAt *time.Time `json:"updated_at"`
}

// JSONPayload implements Payload
func (p *CommitStatusPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// WorkflowJobPayload represents a payload information of workflow job event.
type WorkflowJobPayload struct {
	Action       string             `json:"action"`
	WorkflowJob  *ActionWorkflowJob `json:"workflow_job"`
	PullRequest  *PullRequest       `json:"pull_request,omitempty"`
	Organization *Organization      `json:"organization,omitempty"`
	Repo         *Repository        `json:"repository"`
	Sender       *User              `json:"sender"`
}

// JSONPayload implements Payload
func (p *WorkflowJobPayload) JSONPayload() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
