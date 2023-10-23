// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"xorm.io/builder"
)

// ErrWebhookNotExist represents a "WebhookNotExist" kind of error.
type ErrWebhookNotExist struct {
	ID int64
}

// IsErrWebhookNotExist checks if an error is a ErrWebhookNotExist.
func IsErrWebhookNotExist(err error) bool {
	_, ok := err.(ErrWebhookNotExist)
	return ok
}

func (err ErrWebhookNotExist) Error() string {
	return fmt.Sprintf("webhook does not exist [id: %d]", err.ID)
}

func (err ErrWebhookNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrHookTaskNotExist represents a "HookTaskNotExist" kind of error.
type ErrHookTaskNotExist struct {
	TaskID int64
	HookID int64
	UUID   string
}

// IsErrHookTaskNotExist checks if an error is a ErrHookTaskNotExist.
func IsErrHookTaskNotExist(err error) bool {
	_, ok := err.(ErrHookTaskNotExist)
	return ok
}

func (err ErrHookTaskNotExist) Error() string {
	return fmt.Sprintf("hook task does not exist [task: %d, hook: %d, uuid: %s]", err.TaskID, err.HookID, err.UUID)
}

func (err ErrHookTaskNotExist) Unwrap() error {
	return util.ErrNotExist
}

// HookContentType is the content type of a web hook
type HookContentType int

const (
	// ContentTypeJSON is a JSON payload for web hooks
	ContentTypeJSON HookContentType = iota + 1
	// ContentTypeForm is an url-encoded form payload for web hook
	ContentTypeForm
)

var hookContentTypes = map[string]HookContentType{
	"json": ContentTypeJSON,
	"form": ContentTypeForm,
}

// ToHookContentType returns HookContentType by given name.
func ToHookContentType(name string) HookContentType {
	return hookContentTypes[name]
}

// HookTaskCleanupType is the type of cleanup to perform on hook_task
type HookTaskCleanupType int

const (
	// OlderThan hook_task rows will be cleaned up by the age of the row
	OlderThan HookTaskCleanupType = iota
	// PerWebhook hook_task rows will be cleaned up by leaving the most recent deliveries for each webhook
	PerWebhook
)

var hookTaskCleanupTypes = map[string]HookTaskCleanupType{
	"OlderThan":  OlderThan,
	"PerWebhook": PerWebhook,
}

// ToHookTaskCleanupType returns HookTaskCleanupType by given name.
func ToHookTaskCleanupType(name string) HookTaskCleanupType {
	return hookTaskCleanupTypes[name]
}

// Name returns the name of a given web hook's content type
func (t HookContentType) Name() string {
	switch t {
	case ContentTypeJSON:
		return "json"
	case ContentTypeForm:
		return "form"
	}
	return ""
}

// IsValidHookContentType returns true if given name is a valid hook content type.
func IsValidHookContentType(name string) bool {
	_, ok := hookContentTypes[name]
	return ok
}

// Webhook represents a web hook object.
type Webhook struct {
	ID                        int64 `xorm:"pk autoincr"`
	RepoID                    int64 `xorm:"INDEX"` // An ID of 0 indicates either a default or system webhook
	OwnerID                   int64 `xorm:"INDEX"`
	IsSystemWebhook           bool
	URL                       string `xorm:"url TEXT"`
	HTTPMethod                string `xorm:"http_method"`
	ContentType               HookContentType
	Secret                    string `xorm:"TEXT"`
	Events                    string `xorm:"TEXT"`
	*webhook_module.HookEvent `xorm:"-"`
	IsActive                  bool                      `xorm:"INDEX"`
	Type                      webhook_module.HookType   `xorm:"VARCHAR(16) 'type'"`
	Meta                      string                    `xorm:"TEXT"` // store hook-specific attributes
	LastStatus                webhook_module.HookStatus // Last delivery status

	// HeaderAuthorizationEncrypted should be accessed using HeaderAuthorization() and SetHeaderAuthorization()
	HeaderAuthorizationEncrypted string `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Webhook))
}

// AfterLoad updates the webhook object upon setting a column
func (w *Webhook) AfterLoad() {
	w.HookEvent = &webhook_module.HookEvent{}
	if err := json.Unmarshal([]byte(w.Events), w.HookEvent); err != nil {
		log.Error("Unmarshal[%d]: %v", w.ID, err)
	}
}

// History returns history of webhook by given conditions.
func (w *Webhook) History(ctx context.Context, page int) ([]*HookTask, error) {
	return HookTasks(ctx, w.ID, page)
}

// UpdateEvent handles conversion from HookEvent to Events.
func (w *Webhook) UpdateEvent() error {
	data, err := json.Marshal(w.HookEvent)
	w.Events = string(data)
	return err
}

// HasCreateEvent returns true if hook enabled create event.
func (w *Webhook) HasCreateEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Create)
}

// HasDeleteEvent returns true if hook enabled delete event.
func (w *Webhook) HasDeleteEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Delete)
}

// HasForkEvent returns true if hook enabled fork event.
func (w *Webhook) HasForkEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Fork)
}

// HasIssuesEvent returns true if hook enabled issues event.
func (w *Webhook) HasIssuesEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Issues)
}

// HasIssuesAssignEvent returns true if hook enabled issues assign event.
func (w *Webhook) HasIssuesAssignEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueAssign)
}

// HasIssuesLabelEvent returns true if hook enabled issues label event.
func (w *Webhook) HasIssuesLabelEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueLabel)
}

// HasIssuesMilestoneEvent returns true if hook enabled issues milestone event.
func (w *Webhook) HasIssuesMilestoneEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueMilestone)
}

// HasIssueCommentEvent returns true if hook enabled issue_comment event.
func (w *Webhook) HasIssueCommentEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueComment)
}

// HasPushEvent returns true if hook enabled push event.
func (w *Webhook) HasPushEvent() bool {
	return w.PushOnly || w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Push)
}

// HasPullRequestEvent returns true if hook enabled pull request event.
func (w *Webhook) HasPullRequestEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequest)
}

// HasPullRequestAssignEvent returns true if hook enabled pull request assign event.
func (w *Webhook) HasPullRequestAssignEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestAssign)
}

// HasPullRequestLabelEvent returns true if hook enabled pull request label event.
func (w *Webhook) HasPullRequestLabelEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestLabel)
}

// HasPullRequestMilestoneEvent returns true if hook enabled pull request milestone event.
func (w *Webhook) HasPullRequestMilestoneEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestMilestone)
}

// HasPullRequestCommentEvent returns true if hook enabled pull_request_comment event.
func (w *Webhook) HasPullRequestCommentEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestComment)
}

// HasPullRequestApprovedEvent returns true if hook enabled pull request review event.
func (w *Webhook) HasPullRequestApprovedEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestReview)
}

// HasPullRequestRejectedEvent returns true if hook enabled pull request review event.
func (w *Webhook) HasPullRequestRejectedEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestReview)
}

// HasPullRequestReviewCommentEvent returns true if hook enabled pull request review event.
func (w *Webhook) HasPullRequestReviewCommentEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestReview)
}

// HasPullRequestSyncEvent returns true if hook enabled pull request sync event.
func (w *Webhook) HasPullRequestSyncEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestSync)
}

// HasWikiEvent returns true if hook enabled wiki event.
func (w *Webhook) HasWikiEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvent.Wiki)
}

// HasReleaseEvent returns if hook enabled release event.
func (w *Webhook) HasReleaseEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Release)
}

// HasRepositoryEvent returns if hook enabled repository event.
func (w *Webhook) HasRepositoryEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Repository)
}

// HasPackageEvent returns if hook enabled package event.
func (w *Webhook) HasPackageEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Package)
}

// HasPullRequestReviewRequestEvent returns true if hook enabled pull request review request event.
func (w *Webhook) HasPullRequestReviewRequestEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestReviewRequest)
}

// EventCheckers returns event checkers
func (w *Webhook) EventCheckers() []struct {
	Has  func() bool
	Type webhook_module.HookEventType
} {
	return []struct {
		Has  func() bool
		Type webhook_module.HookEventType
	}{
		{w.HasCreateEvent, webhook_module.HookEventCreate},
		{w.HasDeleteEvent, webhook_module.HookEventDelete},
		{w.HasForkEvent, webhook_module.HookEventFork},
		{w.HasPushEvent, webhook_module.HookEventPush},
		{w.HasIssuesEvent, webhook_module.HookEventIssues},
		{w.HasIssuesAssignEvent, webhook_module.HookEventIssueAssign},
		{w.HasIssuesLabelEvent, webhook_module.HookEventIssueLabel},
		{w.HasIssuesMilestoneEvent, webhook_module.HookEventIssueMilestone},
		{w.HasIssueCommentEvent, webhook_module.HookEventIssueComment},
		{w.HasPullRequestEvent, webhook_module.HookEventPullRequest},
		{w.HasPullRequestAssignEvent, webhook_module.HookEventPullRequestAssign},
		{w.HasPullRequestLabelEvent, webhook_module.HookEventPullRequestLabel},
		{w.HasPullRequestMilestoneEvent, webhook_module.HookEventPullRequestMilestone},
		{w.HasPullRequestCommentEvent, webhook_module.HookEventPullRequestComment},
		{w.HasPullRequestApprovedEvent, webhook_module.HookEventPullRequestReviewApproved},
		{w.HasPullRequestRejectedEvent, webhook_module.HookEventPullRequestReviewRejected},
		{w.HasPullRequestCommentEvent, webhook_module.HookEventPullRequestReviewComment},
		{w.HasPullRequestSyncEvent, webhook_module.HookEventPullRequestSync},
		{w.HasWikiEvent, webhook_module.HookEventWiki},
		{w.HasRepositoryEvent, webhook_module.HookEventRepository},
		{w.HasReleaseEvent, webhook_module.HookEventRelease},
		{w.HasPackageEvent, webhook_module.HookEventPackage},
		{w.HasPullRequestReviewRequestEvent, webhook_module.HookEventPullRequestReviewRequest},
	}
}

// EventsArray returns an array of hook events
func (w *Webhook) EventsArray() []string {
	events := make([]string, 0, 7)

	for _, c := range w.EventCheckers() {
		if c.Has() {
			events = append(events, string(c.Type))
		}
	}
	return events
}

// HeaderAuthorization returns the decrypted Authorization header.
// Not on the reference (*w), to be accessible on WebhooksNew.
func (w Webhook) HeaderAuthorization() (string, error) {
	if w.HeaderAuthorizationEncrypted == "" {
		return "", nil
	}
	return secret.DecryptSecret(setting.SecretKey, w.HeaderAuthorizationEncrypted)
}

// SetHeaderAuthorization encrypts and sets the Authorization header.
func (w *Webhook) SetHeaderAuthorization(cleartext string) error {
	if cleartext == "" {
		w.HeaderAuthorizationEncrypted = ""
		return nil
	}
	ciphertext, err := secret.EncryptSecret(setting.SecretKey, cleartext)
	if err != nil {
		return err
	}
	w.HeaderAuthorizationEncrypted = ciphertext
	return nil
}

// CreateWebhook creates a new web hook.
func CreateWebhook(ctx context.Context, w *Webhook) error {
	w.Type = strings.TrimSpace(w.Type)
	return db.Insert(ctx, w)
}

// CreateWebhooks creates multiple web hooks
func CreateWebhooks(ctx context.Context, ws []*Webhook) error {
	// xorm returns err "no element on slice when insert" for empty slices.
	if len(ws) == 0 {
		return nil
	}
	for i := 0; i < len(ws); i++ {
		ws[i].Type = strings.TrimSpace(ws[i].Type)
	}
	return db.Insert(ctx, ws)
}

// getWebhook uses argument bean as query condition,
// ID must be specified and do not assign unnecessary fields.
func getWebhook(ctx context.Context, bean *Webhook) (*Webhook, error) {
	has, err := db.GetEngine(ctx).Get(bean)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{ID: bean.ID}
	}
	return bean, nil
}

// GetWebhookByID returns webhook of repository by given ID.
func GetWebhookByID(ctx context.Context, id int64) (*Webhook, error) {
	return getWebhook(ctx, &Webhook{
		ID: id,
	})
}

// GetWebhookByRepoID returns webhook of repository by given ID.
func GetWebhookByRepoID(ctx context.Context, repoID, id int64) (*Webhook, error) {
	return getWebhook(ctx, &Webhook{
		ID:     id,
		RepoID: repoID,
	})
}

// GetWebhookByOwnerID returns webhook of a user or organization by given ID.
func GetWebhookByOwnerID(ctx context.Context, ownerID, id int64) (*Webhook, error) {
	return getWebhook(ctx, &Webhook{
		ID:      id,
		OwnerID: ownerID,
	})
}

// ListWebhookOptions are options to filter webhooks on ListWebhooksByOpts
type ListWebhookOptions struct {
	db.ListOptions
	RepoID   int64
	OwnerID  int64
	IsActive util.OptionalBool
}

func (opts *ListWebhookOptions) toCond() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"webhook.repo_id": opts.RepoID})
	}
	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"webhook.owner_id": opts.OwnerID})
	}
	if !opts.IsActive.IsNone() {
		cond = cond.And(builder.Eq{"webhook.is_active": opts.IsActive.IsTrue()})
	}
	return cond
}

// ListWebhooksByOpts return webhooks based on options
func ListWebhooksByOpts(ctx context.Context, opts *ListWebhookOptions) ([]*Webhook, error) {
	sess := db.GetEngine(ctx).Where(opts.toCond())

	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, opts)
		webhooks := make([]*Webhook, 0, opts.PageSize)
		err := sess.Find(&webhooks)
		return webhooks, err
	}

	webhooks := make([]*Webhook, 0, 10)
	err := sess.Find(&webhooks)
	return webhooks, err
}

// CountWebhooksByOpts count webhooks based on options and ignore pagination
func CountWebhooksByOpts(ctx context.Context, opts *ListWebhookOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toCond()).Count(&Webhook{})
}

// UpdateWebhook updates information of webhook.
func UpdateWebhook(ctx context.Context, w *Webhook) error {
	_, err := db.GetEngine(ctx).ID(w.ID).AllCols().Update(w)
	return err
}

// UpdateWebhookLastStatus updates last status of webhook.
func UpdateWebhookLastStatus(ctx context.Context, w *Webhook) error {
	_, err := db.GetEngine(ctx).ID(w.ID).Cols("last_status").Update(w)
	return err
}

// deleteWebhook uses argument bean as query condition,
// ID must be specified and do not assign unnecessary fields.
func deleteWebhook(ctx context.Context, bean *Webhook) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if count, err := db.DeleteByBean(ctx, bean); err != nil {
		return err
	} else if count == 0 {
		return ErrWebhookNotExist{ID: bean.ID}
	} else if _, err = db.DeleteByBean(ctx, &HookTask{HookID: bean.ID}); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteWebhookByRepoID deletes webhook of repository by given ID.
func DeleteWebhookByRepoID(ctx context.Context, repoID, id int64) error {
	return deleteWebhook(ctx, &Webhook{
		ID:     id,
		RepoID: repoID,
	})
}

// DeleteWebhookByOwnerID deletes webhook of a user or organization by given ID.
func DeleteWebhookByOwnerID(ctx context.Context, ownerID, id int64) error {
	return deleteWebhook(ctx, &Webhook{
		ID:      id,
		OwnerID: ownerID,
	})
}
