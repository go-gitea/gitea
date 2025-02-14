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
	"code.gitea.io/gitea/modules/optional"
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

func (w *Webhook) HasEvent(evt webhook_module.HookEventType) bool {
	if w.SendEverything {
		return true
	}
	if w.PushOnly {
		return evt == webhook_module.HookEventPush
	}
	checkEvt := evt
	switch evt {
	case webhook_module.HookEventPullRequestReviewApproved, webhook_module.HookEventPullRequestReviewRejected, webhook_module.HookEventPullRequestReviewComment:
		checkEvt = webhook_module.HookEventPullRequestReview
	}
	return w.HookEvents[checkEvt]
}

// EventsArray returns an array of hook events
func (w *Webhook) EventsArray() []string {
	if w.SendEverything {
		events := make([]string, 0, len(webhook_module.AllEvents()))
		for _, evt := range webhook_module.AllEvents() {
			events = append(events, string(evt))
		}
		return events
	}

	if w.PushOnly {
		return []string{string(webhook_module.HookEventPush)}
	}

	events := make([]string, 0, len(w.HookEvents))
	for event, enabled := range w.HookEvents {
		if enabled {
			events = append(events, string(event))
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

// GetWebhookByID returns webhook of repository by given ID.
func GetWebhookByID(ctx context.Context, id int64) (*Webhook, error) {
	bean := new(Webhook)
	has, err := db.GetEngine(ctx).ID(id).Get(bean)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{ID: id}
	}
	return bean, nil
}

// GetWebhookByRepoID returns webhook of repository by given ID.
func GetWebhookByRepoID(ctx context.Context, repoID, id int64) (*Webhook, error) {
	webhook := new(Webhook)
	has, err := db.GetEngine(ctx).Where("id=? AND repo_id=?", id, repoID).Get(webhook)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{ID: id}
	}
	return webhook, nil
}

// GetWebhookByOwnerID returns webhook of a user or organization by given ID.
func GetWebhookByOwnerID(ctx context.Context, ownerID, id int64) (*Webhook, error) {
	webhook := new(Webhook)
	has, err := db.GetEngine(ctx).Where("id=? AND owner_id=?", id, ownerID).Get(webhook)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{ID: id}
	}
	return webhook, nil
}

// ListWebhookOptions are options to filter webhooks on ListWebhooksByOpts
type ListWebhookOptions struct {
	db.ListOptions
	RepoID   int64
	OwnerID  int64
	IsActive optional.Option[bool]
}

func (opts ListWebhookOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"webhook.repo_id": opts.RepoID})
	}
	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"webhook.owner_id": opts.OwnerID})
	}
	if opts.IsActive.Has() {
		cond = cond.And(builder.Eq{"webhook.is_active": opts.IsActive.Value()})
	}
	return cond
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

// DeleteWebhookByID uses argument bean as query condition,
// ID must be specified and do not assign unnecessary fields.
func DeleteWebhookByID(ctx context.Context, id int64) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if count, err := db.DeleteByID[Webhook](ctx, id); err != nil {
		return err
	} else if count == 0 {
		return ErrWebhookNotExist{ID: id}
	} else if _, err = db.DeleteByBean(ctx, &HookTask{HookID: id}); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteWebhookByRepoID deletes webhook of repository by given ID.
func DeleteWebhookByRepoID(ctx context.Context, repoID, id int64) error {
	if _, err := GetWebhookByRepoID(ctx, repoID, id); err != nil {
		return err
	}
	return DeleteWebhookByID(ctx, id)
}

// DeleteWebhookByOwnerID deletes webhook of a user or organization by given ID.
func DeleteWebhookByOwnerID(ctx context.Context, ownerID, id int64) error {
	if _, err := GetWebhookByOwnerID(ctx, ownerID, id); err != nil {
		return err
	}
	return DeleteWebhookByID(ctx, id)
}
