// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"strings"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/json"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"
	"gitea.dev/modules/web/middleware"
	"gitea.dev/services/webhook"
)

// CreateRepoForm form for creating repository
type CreateRepoForm struct {
	middleware.FormDefaultValidator
	UID           int64  `binding:"Required"`
	RepoName      string `binding:"Required;AlphaDashDot;MaxSize(100)"`
	Private       bool
	Description   string `binding:"MaxSize(2048)"`
	DefaultBranch string `binding:"GitRefName;MaxSize(100)"`
	AutoInit      bool
	Gitignores    string `binding:"MaxSize(1024)"`
	IssueLabels   string `binding:"MaxSize(255)"`
	License       string `binding:"MaxSize(100)"`
	Readme        string `binding:"MaxSize(255)"`
	Template      bool

	RepoTemplate    int64
	GitContent      bool
	Topics          bool
	GitHooks        bool
	Webhooks        bool
	Avatar          bool
	Labels          bool
	ProtectedBranch bool

	ForkSingleBranch string `binding:"MaxSize(255)"`
	ObjectFormatName string
}

// MigrateRepoForm form for migrating repository
// this is used to interact with web ui
type MigrateRepoForm struct {
	middleware.FormDefaultValidator
	// required: true
	CloneAddr    string                 `json:"clone_addr" binding:"Required"`
	Service      structs.GitServiceType `json:"service"`
	AuthUsername string                 `json:"auth_username"`
	AuthPassword string                 `json:"auth_password"`
	AuthToken    string                 `json:"auth_token"`
	// required: true
	UID int64 `json:"uid" binding:"Required"`
	// required: true
	RepoName       string `json:"repo_name" binding:"Required;AlphaDashDot;MaxSize(100)"`
	Mirror         bool   `json:"mirror"`
	LFS            bool   `json:"lfs"`
	LFSEndpoint    string `json:"lfs_endpoint"`
	Private        bool   `json:"private"`
	Description    string `json:"description" binding:"MaxSize(2048)"`
	Wiki           bool   `json:"wiki"`
	Milestones     bool   `json:"milestones"`
	Labels         bool   `json:"labels"`
	Issues         bool   `json:"issues"`
	PullRequests   bool   `json:"pull_requests"`
	Releases       bool   `json:"releases"`
	MirrorInterval string `json:"mirror_interval"`

	AWSAccessKeyID     string `json:"aws_access_key_id"`
	AWSSecretAccessKey string `json:"aws_secret_access_key"`
}

// RepoSettingForm form for changing repository settings
type RepoSettingForm struct {
	middleware.FormDefaultValidator
	RepoName               string `binding:"Required;AlphaDashDot;MaxSize(100)"`
	Description            string `binding:"MaxSize(2048)"`
	Website                string `binding:"ValidUrl;MaxSize(1024)"`
	Interval               string
	MirrorAddress          string
	MirrorUsername         string
	MirrorPassword         string
	LFS                    bool   `form:"mirror_lfs"`
	LFSEndpoint            string `form:"mirror_lfs_endpoint"`
	PushMirrorID           int64
	PushMirrorAddress      string
	PushMirrorUsername     string
	PushMirrorPassword     string
	PushMirrorSyncOnCommit bool
	PushMirrorInterval     string
	Template               bool
	EnablePrune            bool

	// Advanced settings
	EnableCode bool

	EnableWiki         bool
	EnableExternalWiki bool
	DefaultWikiBranch  string
	ExternalWikiURL    string

	EnableIssues                          bool
	EnableExternalTracker                 bool
	ExternalTrackerURL                    string
	TrackerURLFormat                      string
	TrackerIssueStyle                     string
	ExternalTrackerRegexpPattern          string
	EnableCloseIssuesViaCommitInAnyBranch bool

	EnableProjects bool
	ProjectsMode   string

	EnableReleases bool

	EnablePackages bool

	EnablePulls                      bool
	PullsIgnoreWhitespace            bool
	PullsAllowMerge                  bool
	PullsAllowRebase                 bool
	PullsAllowRebaseMerge            bool
	PullsAllowSquash                 bool
	PullsAllowFastForwardOnly        bool
	PullsAllowManualMerge            bool
	PullsDefaultMergeStyle           string
	EnableAutodetectManualMerge      bool
	PullsAllowMergeUpdate            bool
	PullsAllowRebaseUpdate           bool
	PullsDefaultUpdateStyle          string
	DefaultDeleteBranchAfterMerge    bool
	DefaultAllowMaintainerEdit       bool
	DefaultTargetBranch              string
	EnableTimetracker                bool
	AllowOnlyContributorsToTrackTime bool
	EnableIssueDependencies          bool

	// Signing Settings
	TrustModel string

	// Admin settings
	EnableHealthCheck  bool
	RequestReindexType string
}

// ProtectBranchForm form for changing protected branch settings
type ProtectBranchForm struct {
	middleware.FormDefaultValidator
	RuleName                      string `binding:"Required"`
	RuleID                        int64
	EnablePush                    string
	WhitelistUsers                string
	WhitelistTeams                string
	WhitelistDeployKeys           bool
	EnableForcePush               string
	ForcePushAllowlistUsers       string
	ForcePushAllowlistTeams       string
	ForcePushAllowlistDeployKeys  bool
	EnableMergeWhitelist          bool
	MergeWhitelistUsers           string
	MergeWhitelistTeams           string
	EnableBypassAllowlist         bool
	BypassAllowlistUsers          string
	BypassAllowlistTeams          string
	EnableStatusCheck             bool
	StatusCheckContexts           string
	RequiredApprovals             int64
	EnableApprovalsWhitelist      bool
	ApprovalsWhitelistUsers       string
	ApprovalsWhitelistTeams       string
	BlockOnRejectedReviews        bool
	BlockOnOfficialReviewRequests bool
	BlockOnCodeownerReviews       bool
	BlockOnOutdatedBranch         bool
	DismissStaleApprovals         bool
	IgnoreStaleApprovals          bool
	RequireSignedCommits          bool
	ProtectedFilePatterns         string
	UnprotectedFilePatterns       string
	BlockAdminMergeOverride       bool
}

// WebhookForm form for changing web hook
type WebhookForm struct {
	middleware.FormDefaultValidator
	Name                     string `binding:"MaxSize(255)"`
	Events                   string
	Create                   bool
	Delete                   bool
	Fork                     bool
	Push                     bool
	Issues                   bool
	IssueAssign              bool
	IssueLabel               bool
	IssueMilestone           bool
	IssueComment             bool
	PullRequest              bool
	PullRequestAssign        bool
	PullRequestLabel         bool
	PullRequestMilestone     bool
	PullRequestComment       bool
	PullRequestReview        bool
	PullRequestSync          bool
	PullRequestReviewRequest bool
	Wiki                     bool
	Repository               bool
	Release                  bool
	Package                  bool
	Status                   bool
	WorkflowRun              bool
	WorkflowJob              bool
	Active                   bool
	BranchFilter             string `binding:"GlobPattern"`
	AuthorizationHeader      string
	Secret                   string
}

// PushOnly if the hook will be triggered when push
func (f WebhookForm) PushOnly() bool {
	return f.Events == "push_only"
}

// SendEverything if the hook will be triggered any event
func (f WebhookForm) SendEverything() bool {
	return f.Events == "send_everything"
}

// ChooseEvents if the hook will be triggered choose events
func (f WebhookForm) ChooseEvents() bool {
	return f.Events == "choose_events"
}

// NewWebhookForm form for creating web hook
type NewWebhookForm struct {
	middleware.FormDefaultValidator
	PayloadURL  string `binding:"Required;ValidUrl"`
	HTTPMethod  string `binding:"Required;In(POST,GET)"`
	ContentType int    `binding:"Required"`
	WebhookForm
}

// NewGogshookForm form for creating gogs hook
type NewGogshookForm struct {
	middleware.FormDefaultValidator
	PayloadURL  string `binding:"Required;ValidUrl"`
	ContentType int    `binding:"Required"`
	WebhookForm
}

// NewSlackHookForm form for creating slack hook
type NewSlackHookForm struct {
	PayloadURL string `binding:"Required;ValidUrl"`
	Channel    string `binding:"Required"`
	Username   string
	IconURL    string
	Color      string
	WebhookForm
}

func (f *NewSlackHookForm) Validate(ctx *middleware.ValidateContext, errs validation.BindingErrors) validation.BindingErrors {
	if !webhook.IsValidSlackChannel(strings.TrimSpace(f.Channel)) {
		errs = middleware.AddValidationError(errs, "Channel", ctx.Locale.TrString("repo.settings.add_webhook.invalid_channel_name"))
	}
	return errs
}

// NewDiscordHookForm form for creating discord hook
type NewDiscordHookForm struct {
	middleware.FormDefaultValidator
	PayloadURL string `binding:"Required;ValidUrl"`
	Username   string
	IconURL    string
	WebhookForm
}

// NewDingtalkHookForm form for creating dingtalk hook
type NewDingtalkHookForm struct {
	middleware.FormDefaultValidator
	PayloadURL string `binding:"Required;ValidUrl"`
	WebhookForm
}

// NewTelegramHookForm form for creating telegram hook
type NewTelegramHookForm struct {
	middleware.FormDefaultValidator
	BotToken string `binding:"Required"`
	ChatID   string `binding:"Required"`
	ThreadID string
	WebhookForm
}

// NewMatrixHookForm form for creating Matrix hook
type NewMatrixHookForm struct {
	middleware.FormDefaultValidator
	HomeserverURL string `binding:"Required;ValidUrl"`
	RoomID        string `binding:"Required"`
	MessageType   int
	WebhookForm
}

// NewMSTeamsHookForm form for creating MS Teams hook
type NewMSTeamsHookForm struct {
	middleware.FormDefaultValidator
	PayloadURL string `binding:"Required;ValidUrl"`
	WebhookForm
}

// NewFeishuHookForm form for creating feishu hook
type NewFeishuHookForm struct {
	middleware.FormDefaultValidator
	PayloadURL string `binding:"Required;ValidUrl"`
	WebhookForm
}

// NewWechatWorkHookForm form for creating wechatwork hook
type NewWechatWorkHookForm struct {
	middleware.FormDefaultValidator
	PayloadURL string `binding:"Required;ValidUrl"`
	WebhookForm
}

// NewPackagistHookForm form for creating packagist hook
type NewPackagistHookForm struct {
	middleware.FormDefaultValidator
	Username   string `binding:"Required"`
	APIToken   string `binding:"Required"`
	PackageURL string `binding:"Required;ValidUrl"`
	WebhookForm
}

// CreateIssueForm form for creating issue
type CreateIssueForm struct {
	middleware.FormDefaultValidator
	Title               string `binding:"TrimSpace;Required;MaxSize(255)"`
	AssigneeIDs         string `form:"assignee_ids"`
	ReviewerIDs         string `form:"reviewer_ids"`
	Ref                 string `form:"ref"`
	MilestoneID         int64
	Content             string
	Files               []string
	AllowMaintainerEdit bool
}

// CreateCommentForm form for creating comment
type CreateCommentForm struct {
	middleware.FormDefaultValidator
	Content string
	Status  string `binding:"OmitEmpty;In(reopen,close)"`
	Files   []string
}

// ReactionForm form for adding and removing reaction
type ReactionForm struct {
	middleware.FormDefaultValidator
	Content string `binding:"Required"`
}

// IssueLockForm form for locking an issue
type IssueLockForm struct {
	middleware.FormDefaultValidator
	Reason string `binding:"Required"`
}

// CreateProjectForm form for creating a project
type CreateProjectForm struct {
	middleware.FormDefaultValidator
	Title        string `binding:"Required;MaxSize(100)"`
	Content      string
	TemplateType project_model.TemplateType
	CardType     project_model.CardType
}

// EditProjectColumnForm is a form for editing a project column
type EditProjectColumnForm struct {
	middleware.FormDefaultValidator
	Title   string `binding:"Required;MaxSize(100)"`
	Sorting int8
	Color   string `binding:"MaxSize(7)"`
}

// CreateMilestoneForm form for creating milestone
type CreateMilestoneForm struct {
	middleware.FormDefaultValidator
	Title    string `binding:"Required;MaxSize(50)"`
	Content  string
	Deadline string
}

// CreateLabelForm form for creating label
type CreateLabelForm struct {
	middleware.FormDefaultValidator
	ID             int64
	Title          string `binding:"Required;MaxSize(50)" locale:"repo.issues.label_title"`
	Exclusive      bool   `form:"exclusive"`
	ExclusiveOrder int    `form:"exclusive_order"`
	IsArchived     bool   `form:"is_archived"`
	Description    string `binding:"MaxSize(200)" locale:"repo.issues.label_description"`
	Color          string `binding:"Required;MaxSize(7)" locale:"repo.issues.label_color"`
}

// InitializeLabelsForm form for initializing labels
type InitializeLabelsForm struct {
	middleware.FormDefaultValidator
	TemplateName string `binding:"Required"`
}

// MergePullRequestForm form for merging Pull Request
// swagger:model MergePullRequestOption
type MergePullRequestForm struct {
	middleware.FormDefaultValidator
	// required: true
	// enum: ["merge","rebase","rebase-merge","squash","fast-forward-only","manually-merged"]
	Do                     string `json:"do" binding:"Required;In(merge,rebase,rebase-merge,squash,fast-forward-only,manually-merged)"`
	MergeTitleField        string `json:"merge_title_field,omitempty"`
	MergeMessageField      string `json:"merge_message_field,omitempty"`
	MergeCommitID          string `json:"merge_commit_id,omitempty"` // only used for manually-merged
	HeadCommitID           string `json:"head_commit_id,omitempty"`
	ForceMerge             bool   `json:"force_merge,omitempty"`
	MergeWhenChecksSucceed bool   `json:"merge_when_checks_succeed,omitempty"`
	DeleteBranchAfterMerge *bool  `json:"delete_branch_after_merge,omitempty"`
}

func (f *MergePullRequestForm) UnmarshalJSON(b []byte) error {
	// This is for backward compatibility, to support both field names like "do" and "Do",
	// because old code doesn't have "json" tag for these fields
	type aux struct {
		Do1                string `json:"do"`
		Do2                string `json:"Do"`
		MergeTitleField1   string `json:"merge_title_field"`
		MergeTitleField2   string `json:"MergeTitleField"`
		MergeMessageField1 string `json:"merge_message_field"`
		MergeMessageField2 string `json:"MergeMessageField"`
		MergeCommitID1     string `json:"merge_commit_id"`
		MergeCommitID2     string `json:"MergeCommitID"`

		HeadCommitID           string `json:"head_commit_id"`
		ForceMerge             bool   `json:"force_merge"`
		MergeWhenChecksSucceed bool   `json:"merge_when_checks_succeed"`
		DeleteBranchAfterMerge *bool  `json:"delete_branch_after_merge"`
	}
	var a aux
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	f.Do = util.IfZero(a.Do1, a.Do2)
	f.MergeTitleField = util.IfZero(a.MergeTitleField1, a.MergeTitleField2)
	f.MergeMessageField = util.IfZero(a.MergeMessageField1, a.MergeMessageField2)
	f.MergeCommitID = util.IfZero(a.MergeCommitID1, a.MergeCommitID2)
	f.HeadCommitID = a.HeadCommitID
	f.ForceMerge = a.ForceMerge
	f.MergeWhenChecksSucceed = a.MergeWhenChecksSucceed
	f.DeleteBranchAfterMerge = a.DeleteBranchAfterMerge
	return nil
}

// CodeCommentForm form for adding code comments for PRs
type CodeCommentForm struct {
	middleware.FormDefaultValidator
	Origin         string `binding:"Required;In(timeline,diff)"`
	Content        string `binding:"Required"`
	Side           string `binding:"Required;In(previous,proposed)"`
	Line           int64
	TreePath       string `form:"path" binding:"Required"`
	SingleReview   bool   `form:"single_review"`
	Reply          int64  `form:"reply"`
	LatestCommitID string
	Files          []string
}

// SubmitReviewForm for submitting a finished code review
type SubmitReviewForm struct {
	middleware.FormDefaultValidator
	Content  string
	Type     string
	CommitID string
	Files    []string
}

// ReviewType will return the corresponding ReviewType for type
func (f SubmitReviewForm) ReviewType() issues_model.ReviewType {
	switch f.Type {
	case "approve":
		return issues_model.ReviewTypeApprove
	case "comment":
		return issues_model.ReviewTypeComment
	case "reject":
		return issues_model.ReviewTypeReject
	case "":
		return issues_model.ReviewTypeComment // default to comment when doing quick-submit (Ctrl+Enter) on the review form
	default:
		return issues_model.ReviewTypeUnknown
	}
}

// HasEmptyContent checks if the content of the review form is empty.
func (f SubmitReviewForm) HasEmptyContent() bool {
	reviewType := f.ReviewType()

	return (reviewType == issues_model.ReviewTypeComment || reviewType == issues_model.ReviewTypeReject) &&
		len(strings.TrimSpace(f.Content)) == 0
}

// DismissReviewForm for dismissing stale review by repo admin
type DismissReviewForm struct {
	middleware.FormDefaultValidator
	ReviewID int64 `binding:"Required"`
	Message  string
}

// UpdateAllowEditsForm form for changing if PR allows edits from maintainers
type UpdateAllowEditsForm struct {
	middleware.FormDefaultValidator
	AllowMaintainerEdit bool
}

// __________       .__
// \______   \ ____ |  |   ____ _____    ______ ____
//  |       _// __ \|  | _/ __ \\__  \  /  ___// __ \
//  |    |   \  ___/|  |_\  ___/ / __ \_\___ \\  ___/
//  |____|_  /\___  >____/\___  >____  /____  >\___  >
//         \/     \/          \/     \/     \/     \/

// NewReleaseForm form for creating release
type NewReleaseForm struct {
	middleware.FormDefaultValidator
	TagName    string `binding:"Required;GitRefName;MaxSize(255)"`
	Target     string `form:"tag_target" binding:"Required;MaxSize(255)"`
	Title      string `binding:"MaxSize(255)"`
	Content    string
	Draft      bool
	TagOnly    bool
	Prerelease bool
	AddTagMsg  bool
	Files      []string
}

// GenerateReleaseNotesForm retrieves release notes recommendations.
type GenerateReleaseNotesForm struct {
	middleware.FormDefaultValidator
	TagName     string `form:"tag_name" binding:"Required;GitRefName;MaxSize(255)"`
	TagTarget   string `form:"tag_target" binding:"MaxSize(255)"`
	PreviousTag string `form:"previous_tag" binding:"MaxSize(255)"`
}

// EditReleaseForm form for changing release
type EditReleaseForm struct {
	middleware.FormDefaultValidator
	Title      string `form:"title" binding:"TrimSpace;Required;MaxSize(255)"`
	Content    string `form:"content"`
	Draft      string `form:"draft"`
	Prerelease bool   `form:"prerelease"`
	Files      []string
}

type WikiEditForm struct {
	middleware.FormDefaultValidator
	Title   string `binding:"TrimSpace;Required"`
	Content string
	Message string
}

// AddTimeManuallyForm form that adds spent time manually.
type AddTimeManuallyForm struct {
	middleware.FormDefaultValidator
	Hours   int `binding:"Range(0,1000)"`
	Minutes int `binding:"Range(0,1000)"`
}

// SaveTopicForm form for save topics for repository
type SaveTopicForm struct {
	middleware.FormDefaultValidator
	Topics []string `binding:"topics;Required;"`
}
