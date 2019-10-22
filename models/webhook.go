// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/gobwas/glob"
	gouuid "github.com/satori/go.uuid"
	"github.com/unknwon/com"
)

// HookQueue is a global queue of web hooks
var HookQueue = sync.NewUniqueQueue(setting.Webhook.QueueLength)

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

// HookEvents is a set of web hook events
type HookEvents struct {
	Create       bool `json:"create"`
	Delete       bool `json:"delete"`
	Fork         bool `json:"fork"`
	Issues       bool `json:"issues"`
	IssueComment bool `json:"issue_comment"`
	Push         bool `json:"push"`
	PullRequest  bool `json:"pull_request"`
	Repository   bool `json:"repository"`
	Release      bool `json:"release"`
}

// HookEvent represents events that will delivery hook.
type HookEvent struct {
	PushOnly       bool   `json:"push_only"`
	SendEverything bool   `json:"send_everything"`
	ChooseEvents   bool   `json:"choose_events"`
	BranchFilter   string `json:"branch_filter"`

	HookEvents `json:"events"`
}

// HookStatus is the status of a web hook
type HookStatus int

// Possible statuses of a web hook
const (
	HookStatusNone = iota
	HookStatusSucceed
	HookStatusFail
)

// Webhook represents a web hook object.
type Webhook struct {
	ID           int64  `xorm:"pk autoincr"`
	RepoID       int64  `xorm:"INDEX"`
	OrgID        int64  `xorm:"INDEX"`
	URL          string `xorm:"url TEXT"`
	Signature    string `xorm:"TEXT"`
	HTTPMethod   string `xorm:"http_method"`
	ContentType  HookContentType
	Secret       string `xorm:"TEXT"`
	Events       string `xorm:"TEXT"`
	*HookEvent   `xorm:"-"`
	IsSSL        bool `xorm:"is_ssl"`
	IsActive     bool `xorm:"INDEX"`
	HookTaskType HookTaskType
	Meta         string     `xorm:"TEXT"` // store hook-specific attributes
	LastStatus   HookStatus // Last delivery status

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// AfterLoad updates the webhook object upon setting a column
func (w *Webhook) AfterLoad() {
	w.HookEvent = &HookEvent{}
	if err := json.Unmarshal([]byte(w.Events), w.HookEvent); err != nil {
		log.Error("Unmarshal[%d]: %v", w.ID, err)
	}
}

// GetSlackHook returns slack metadata
func (w *Webhook) GetSlackHook() *SlackMeta {
	s := &SlackMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetSlackHook(%d): %v", w.ID, err)
	}
	return s
}

// GetDiscordHook returns discord metadata
func (w *Webhook) GetDiscordHook() *DiscordMeta {
	s := &DiscordMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetDiscordHook(%d): %v", w.ID, err)
	}
	return s
}

// GetTelegramHook returns telegram metadata
func (w *Webhook) GetTelegramHook() *TelegramMeta {
	s := &TelegramMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetTelegramHook(%d): %v", w.ID, err)
	}
	return s
}

// History returns history of webhook by given conditions.
func (w *Webhook) History(page int) ([]*HookTask, error) {
	return HookTasks(w.ID, page)
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

func (w *Webhook) eventCheckers() []struct {
	has func() bool
	typ HookEventType
} {
	return []struct {
		has func() bool
		typ HookEventType
	}{
		{w.HasCreateEvent, HookEventCreate},
		{w.HasDeleteEvent, HookEventDelete},
		{w.HasForkEvent, HookEventFork},
		{w.HasPushEvent, HookEventPush},
		{w.HasIssuesEvent, HookEventIssues},
		{w.HasIssueCommentEvent, HookEventIssueComment},
		{w.HasPullRequestEvent, HookEventPullRequest},
		{w.HasRepositoryEvent, HookEventRepository},
		{w.HasReleaseEvent, HookEventRelease},
	}
}

// EventsArray returns an array of hook events
func (w *Webhook) EventsArray() []string {
	events := make([]string, 0, 7)

	for _, c := range w.eventCheckers() {
		if c.has() {
			events = append(events, string(c.typ))
		}
	}
	return events
}

func (w *Webhook) checkBranch(branch string) bool {
	if w.BranchFilter == "" || w.BranchFilter == "*" {
		return true
	}

	g, err := glob.Compile(w.BranchFilter)
	if err != nil {
		// should not really happen as BranchFilter is validated
		log.Error("CheckBranch failed: %s", err)
		return false
	}

	return g.Match(branch)
}

// CreateWebhook creates a new web hook.
func CreateWebhook(w *Webhook) error {
	return createWebhook(x, w)
}

func createWebhook(e Engine, w *Webhook) error {
	_, err := e.Insert(w)
	return err
}

// getWebhook uses argument bean as query condition,
// ID must be specified and do not assign unnecessary fields.
func getWebhook(bean *Webhook) (*Webhook, error) {
	has, err := x.Get(bean)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{bean.ID}
	}
	return bean, nil
}

// GetWebhookByID returns webhook of repository by given ID.
func GetWebhookByID(id int64) (*Webhook, error) {
	return getWebhook(&Webhook{
		ID: id,
	})
}

// GetWebhookByRepoID returns webhook of repository by given ID.
func GetWebhookByRepoID(repoID, id int64) (*Webhook, error) {
	return getWebhook(&Webhook{
		ID:     id,
		RepoID: repoID,
	})
}

// GetWebhookByOrgID returns webhook of organization by given ID.
func GetWebhookByOrgID(orgID, id int64) (*Webhook, error) {
	return getWebhook(&Webhook{
		ID:    id,
		OrgID: orgID,
	})
}

// GetActiveWebhooksByRepoID returns all active webhooks of repository.
func GetActiveWebhooksByRepoID(repoID int64) ([]*Webhook, error) {
	return getActiveWebhooksByRepoID(x, repoID)
}

func getActiveWebhooksByRepoID(e Engine, repoID int64) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, e.Where("is_active=?", true).
		Find(&webhooks, &Webhook{RepoID: repoID})
}

// GetWebhooksByRepoID returns all webhooks of a repository.
func GetWebhooksByRepoID(repoID int64) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, x.Find(&webhooks, &Webhook{RepoID: repoID})
}

// GetActiveWebhooksByOrgID returns all active webhooks for an organization.
func GetActiveWebhooksByOrgID(orgID int64) (ws []*Webhook, err error) {
	return getActiveWebhooksByOrgID(x, orgID)
}

func getActiveWebhooksByOrgID(e Engine, orgID int64) (ws []*Webhook, err error) {
	err = e.
		Where("org_id=?", orgID).
		And("is_active=?", true).
		Find(&ws)
	return ws, err
}

// GetWebhooksByOrgID returns all webhooks for an organization.
func GetWebhooksByOrgID(orgID int64) (ws []*Webhook, err error) {
	err = x.Find(&ws, &Webhook{OrgID: orgID})
	return ws, err
}

// GetDefaultWebhook returns admin-default webhook by given ID.
func GetDefaultWebhook(id int64) (*Webhook, error) {
	webhook := &Webhook{ID: id}
	has, err := x.
		Where("repo_id=? AND org_id=?", 0, 0).
		Get(webhook)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{id}
	}
	return webhook, nil
}

// GetDefaultWebhooks returns all admin-default webhooks.
func GetDefaultWebhooks() ([]*Webhook, error) {
	return getDefaultWebhooks(x)
}

func getDefaultWebhooks(e Engine) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, e.
		Where("repo_id=? AND org_id=?", 0, 0).
		Find(&webhooks)
}

// UpdateWebhook updates information of webhook.
func UpdateWebhook(w *Webhook) error {
	_, err := x.ID(w.ID).AllCols().Update(w)
	return err
}

// UpdateWebhookLastStatus updates last status of webhook.
func UpdateWebhookLastStatus(w *Webhook) error {
	_, err := x.ID(w.ID).Cols("last_status").Update(w)
	return err
}

// deleteWebhook uses argument bean as query condition,
// ID must be specified and do not assign unnecessary fields.
func deleteWebhook(bean *Webhook) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if count, err := sess.Delete(bean); err != nil {
		return err
	} else if count == 0 {
		return ErrWebhookNotExist{ID: bean.ID}
	} else if _, err = sess.Delete(&HookTask{HookID: bean.ID}); err != nil {
		return err
	}

	return sess.Commit()
}

// DeleteWebhookByRepoID deletes webhook of repository by given ID.
func DeleteWebhookByRepoID(repoID, id int64) error {
	return deleteWebhook(&Webhook{
		ID:     id,
		RepoID: repoID,
	})
}

// DeleteWebhookByOrgID deletes webhook of organization by given ID.
func DeleteWebhookByOrgID(orgID, id int64) error {
	return deleteWebhook(&Webhook{
		ID:    id,
		OrgID: orgID,
	})
}

// DeleteDefaultWebhook deletes an admin-default webhook by given ID.
func DeleteDefaultWebhook(id int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	count, err := sess.
		Where("repo_id=? AND org_id=?", 0, 0).
		Delete(&Webhook{ID: id})
	if err != nil {
		return err
	} else if count == 0 {
		return ErrWebhookNotExist{ID: id}
	}

	if _, err := sess.Delete(&HookTask{HookID: id}); err != nil {
		return err
	}

	return sess.Commit()
}

// copyDefaultWebhooksToRepo creates copies of the default webhooks in a new repo
func copyDefaultWebhooksToRepo(e Engine, repoID int64) error {
	ws, err := getDefaultWebhooks(e)
	if err != nil {
		return fmt.Errorf("GetDefaultWebhooks: %v", err)
	}

	for _, w := range ws {
		w.ID = 0
		w.RepoID = repoID
		if err := createWebhook(e, w); err != nil {
			return fmt.Errorf("CreateWebhook: %v", err)
		}
	}
	return nil
}

//   ___ ___                __   ___________              __
//  /   |   \  ____   ____ |  | _\__    ___/____    _____|  | __
// /    ~    \/  _ \ /  _ \|  |/ / |    |  \__  \  /  ___/  |/ /
// \    Y    (  <_> |  <_> )    <  |    |   / __ \_\___ \|    <
//  \___|_  / \____/ \____/|__|_ \ |____|  (____  /____  >__|_ \
//        \/                    \/              \/     \/     \/

// HookTaskType is the type of an hook task
type HookTaskType int

// Types of hook tasks
const (
	GOGS HookTaskType = iota + 1
	SLACK
	GITEA
	DISCORD
	DINGTALK
	TELEGRAM
	MSTEAMS
)

var hookTaskTypes = map[string]HookTaskType{
	"gitea":    GITEA,
	"gogs":     GOGS,
	"slack":    SLACK,
	"discord":  DISCORD,
	"dingtalk": DINGTALK,
	"telegram": TELEGRAM,
	"msteams":  MSTEAMS,
}

// ToHookTaskType returns HookTaskType by given name.
func ToHookTaskType(name string) HookTaskType {
	return hookTaskTypes[name]
}

// Name returns the name of an hook task type
func (t HookTaskType) Name() string {
	switch t {
	case GITEA:
		return "gitea"
	case GOGS:
		return "gogs"
	case SLACK:
		return "slack"
	case DISCORD:
		return "discord"
	case DINGTALK:
		return "dingtalk"
	case TELEGRAM:
		return "telegram"
	case MSTEAMS:
		return "msteams"
	}
	return ""
}

// IsValidHookTaskType returns true if given name is a valid hook task type.
func IsValidHookTaskType(name string) bool {
	_, ok := hookTaskTypes[name]
	return ok
}

// HookEventType is the type of an hook event
type HookEventType string

// Types of hook events
const (
	HookEventCreate              HookEventType = "create"
	HookEventDelete              HookEventType = "delete"
	HookEventFork                HookEventType = "fork"
	HookEventPush                HookEventType = "push"
	HookEventIssues              HookEventType = "issues"
	HookEventIssueComment        HookEventType = "issue_comment"
	HookEventPullRequest         HookEventType = "pull_request"
	HookEventRepository          HookEventType = "repository"
	HookEventRelease             HookEventType = "release"
	HookEventPullRequestApproved HookEventType = "pull_request_approved"
	HookEventPullRequestRejected HookEventType = "pull_request_rejected"
	HookEventPullRequestComment  HookEventType = "pull_request_comment"
)

// HookRequest represents hook task request information.
type HookRequest struct {
	Headers map[string]string `json:"headers"`
}

// HookResponse represents hook task response information.
type HookResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// HookTask represents a hook task.
type HookTask struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	HookID          int64
	UUID            string
	Type            HookTaskType
	URL             string `xorm:"TEXT"`
	Signature       string `xorm:"TEXT"`
	api.Payloader   `xorm:"-"`
	PayloadContent  string `xorm:"TEXT"`
	HTTPMethod      string `xorm:"http_method"`
	ContentType     HookContentType
	EventType       HookEventType
	IsSSL           bool
	IsDelivered     bool
	Delivered       int64
	DeliveredString string `xorm:"-"`

	// History info.
	IsSucceed       bool
	RequestContent  string        `xorm:"TEXT"`
	RequestInfo     *HookRequest  `xorm:"-"`
	ResponseContent string        `xorm:"TEXT"`
	ResponseInfo    *HookResponse `xorm:"-"`
}

// BeforeUpdate will be invoked by XORM before updating a record
// representing this object
func (t *HookTask) BeforeUpdate() {
	if t.RequestInfo != nil {
		t.RequestContent = t.simpleMarshalJSON(t.RequestInfo)
	}
	if t.ResponseInfo != nil {
		t.ResponseContent = t.simpleMarshalJSON(t.ResponseInfo)
	}
}

// AfterLoad updates the webhook object upon setting a column
func (t *HookTask) AfterLoad() {
	t.DeliveredString = time.Unix(0, t.Delivered).Format("2006-01-02 15:04:05 MST")

	if len(t.RequestContent) == 0 {
		return
	}

	t.RequestInfo = &HookRequest{}
	if err := json.Unmarshal([]byte(t.RequestContent), t.RequestInfo); err != nil {
		log.Error("Unmarshal RequestContent[%d]: %v", t.ID, err)
	}

	if len(t.ResponseContent) > 0 {
		t.ResponseInfo = &HookResponse{}
		if err := json.Unmarshal([]byte(t.ResponseContent), t.ResponseInfo); err != nil {
			log.Error("Unmarshal ResponseContent[%d]: %v", t.ID, err)
		}
	}
}

func (t *HookTask) simpleMarshalJSON(v interface{}) string {
	p, err := json.Marshal(v)
	if err != nil {
		log.Error("Marshal [%d]: %v", t.ID, err)
	}
	return string(p)
}

// HookTasks returns a list of hook tasks by given conditions.
func HookTasks(hookID int64, page int) ([]*HookTask, error) {
	tasks := make([]*HookTask, 0, setting.Webhook.PagingNum)
	return tasks, x.
		Limit(setting.Webhook.PagingNum, (page-1)*setting.Webhook.PagingNum).
		Where("hook_id=?", hookID).
		Desc("id").
		Find(&tasks)
}

// CreateHookTask creates a new hook task,
// it handles conversion from Payload to PayloadContent.
func CreateHookTask(t *HookTask) error {
	return createHookTask(x, t)
}

func createHookTask(e Engine, t *HookTask) error {
	data, err := t.Payloader.JSONPayload()
	if err != nil {
		return err
	}
	t.UUID = gouuid.NewV4().String()
	t.PayloadContent = string(data)
	_, err = e.Insert(t)
	return err
}

// UpdateHookTask updates information of hook task.
func UpdateHookTask(t *HookTask) error {
	_, err := x.ID(t.ID).AllCols().Update(t)
	return err
}

// PrepareWebhook adds special webhook to task queue for given payload.
func PrepareWebhook(w *Webhook, repo *Repository, event HookEventType, p api.Payloader) error {
	return prepareWebhook(x, w, repo, event, p)
}

// getPayloadBranch returns branch for hook event, if applicable.
func getPayloadBranch(p api.Payloader) string {
	switch pp := p.(type) {
	case *api.CreatePayload:
		if pp.RefType == "branch" {
			return pp.Ref
		}
	case *api.DeletePayload:
		if pp.RefType == "branch" {
			return pp.Ref
		}
	case *api.PushPayload:
		if strings.HasPrefix(pp.Ref, git.BranchPrefix) {
			return pp.Ref[len(git.BranchPrefix):]
		}
	}
	return ""
}

func prepareWebhook(e Engine, w *Webhook, repo *Repository, event HookEventType, p api.Payloader) error {
	for _, e := range w.eventCheckers() {
		if event == e.typ {
			if !e.has() {
				return nil
			}
		}
	}

	// If payload has no associated branch (e.g. it's a new tag, issue, etc.),
	// branch filter has no effect.
	if branch := getPayloadBranch(p); branch != "" {
		if !w.checkBranch(branch) {
			log.Info("Branch %q doesn't match branch filter %q, skipping", branch, w.BranchFilter)
			return nil
		}
	}

	var payloader api.Payloader
	var err error
	// Use separate objects so modifications won't be made on payload on non-Gogs/Gitea type hooks.
	switch w.HookTaskType {
	case SLACK:
		payloader, err = GetSlackPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetSlackPayload: %v", err)
		}
	case DISCORD:
		payloader, err = GetDiscordPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetDiscordPayload: %v", err)
		}
	case DINGTALK:
		payloader, err = GetDingtalkPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetDingtalkPayload: %v", err)
		}
	case TELEGRAM:
		payloader, err = GetTelegramPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetTelegramPayload: %v", err)
		}
	case MSTEAMS:
		payloader, err = GetMSTeamsPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetMSTeamsPayload: %v", err)
		}
	default:
		p.SetSecret(w.Secret)
		payloader = p
	}

	var signature string
	if len(w.Secret) > 0 {
		data, err := payloader.JSONPayload()
		if err != nil {
			log.Error("prepareWebhooks.JSONPayload: %v", err)
		}
		sig := hmac.New(sha256.New, []byte(w.Secret))
		_, err = sig.Write(data)
		if err != nil {
			log.Error("prepareWebhooks.sigWrite: %v", err)
		}
		signature = hex.EncodeToString(sig.Sum(nil))
	}

	if err = createHookTask(e, &HookTask{
		RepoID:      repo.ID,
		HookID:      w.ID,
		Type:        w.HookTaskType,
		URL:         w.URL,
		Signature:   signature,
		Payloader:   payloader,
		HTTPMethod:  w.HTTPMethod,
		ContentType: w.ContentType,
		EventType:   event,
		IsSSL:       w.IsSSL,
	}); err != nil {
		return fmt.Errorf("CreateHookTask: %v", err)
	}
	return nil
}

// PrepareWebhooks adds new webhooks to task queue for given payload.
func PrepareWebhooks(repo *Repository, event HookEventType, p api.Payloader) error {
	return prepareWebhooks(x, repo, event, p)
}

func prepareWebhooks(e Engine, repo *Repository, event HookEventType, p api.Payloader) error {
	ws, err := getActiveWebhooksByRepoID(e, repo.ID)
	if err != nil {
		return fmt.Errorf("GetActiveWebhooksByRepoID: %v", err)
	}

	// check if repo belongs to org and append additional webhooks
	if repo.mustOwner(e).IsOrganization() {
		// get hooks for org
		orgHooks, err := getActiveWebhooksByOrgID(e, repo.OwnerID)
		if err != nil {
			return fmt.Errorf("GetActiveWebhooksByOrgID: %v", err)
		}
		ws = append(ws, orgHooks...)
	}

	if len(ws) == 0 {
		return nil
	}

	for _, w := range ws {
		if err = prepareWebhook(e, w, repo, event, p); err != nil {
			return err
		}
	}
	return nil
}

func (t *HookTask) deliver() error {
	t.IsDelivered = true

	var req *http.Request
	var err error

	switch t.HTTPMethod {
	case "":
		log.Info("HTTP Method for webhook %d empty, setting to POST as default", t.ID)
		fallthrough
	case http.MethodPost:
		switch t.ContentType {
		case ContentTypeJSON:
			req, err = http.NewRequest("POST", t.URL, strings.NewReader(t.PayloadContent))
			if err != nil {
				return err
			}

			req.Header.Set("Content-Type", "application/json")
		case ContentTypeForm:
			var forms = url.Values{
				"payload": []string{t.PayloadContent},
			}

			req, err = http.NewRequest("POST", t.URL, strings.NewReader(forms.Encode()))
			if err != nil {

				return err
			}

			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	case http.MethodGet:
		u, err := url.Parse(t.URL)
		if err != nil {
			return err
		}
		vals := u.Query()
		vals["payload"] = []string{t.PayloadContent}
		u.RawQuery = vals.Encode()
		req, err = http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid http method for webhook: [%d] %v", t.ID, t.HTTPMethod)
	}

	req.Header.Add("X-Gitea-Delivery", t.UUID)
	req.Header.Add("X-Gitea-Event", string(t.EventType))
	req.Header.Add("X-Gitea-Signature", t.Signature)
	req.Header.Add("X-Gogs-Delivery", t.UUID)
	req.Header.Add("X-Gogs-Event", string(t.EventType))
	req.Header.Add("X-Gogs-Signature", t.Signature)
	req.Header["X-GitHub-Delivery"] = []string{t.UUID}
	req.Header["X-GitHub-Event"] = []string{string(t.EventType)}

	// Record delivery information.
	t.RequestInfo = &HookRequest{
		Headers: map[string]string{},
	}
	for k, vals := range req.Header {
		t.RequestInfo.Headers[k] = strings.Join(vals, ",")
	}

	t.ResponseInfo = &HookResponse{
		Headers: map[string]string{},
	}

	defer func() {
		t.Delivered = time.Now().UnixNano()
		if t.IsSucceed {
			log.Trace("Hook delivered: %s", t.UUID)
		} else {
			log.Trace("Hook delivery failed: %s", t.UUID)
		}

		if err := UpdateHookTask(t); err != nil {
			log.Error("UpdateHookTask [%d]: %v", t.ID, err)
		}

		// Update webhook last delivery status.
		w, err := GetWebhookByID(t.HookID)
		if err != nil {
			log.Error("GetWebhookByID: %v", err)
			return
		}
		if t.IsSucceed {
			w.LastStatus = HookStatusSucceed
		} else {
			w.LastStatus = HookStatusFail
		}
		if err = UpdateWebhookLastStatus(w); err != nil {
			log.Error("UpdateWebhookLastStatus: %v", err)
			return
		}
	}()

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		t.ResponseInfo.Body = fmt.Sprintf("Delivery: %v", err)
		return err
	}
	defer resp.Body.Close()

	// Status code is 20x can be seen as succeed.
	t.IsSucceed = resp.StatusCode/100 == 2
	t.ResponseInfo.Status = resp.StatusCode
	for k, vals := range resp.Header {
		t.ResponseInfo.Headers[k] = strings.Join(vals, ",")
	}

	p, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.ResponseInfo.Body = fmt.Sprintf("read body: %s", err)
		return err
	}
	t.ResponseInfo.Body = string(p)
	return nil
}

// DeliverHooks checks and delivers undelivered hooks.
// TODO: shoot more hooks at same time.
func DeliverHooks() {
	tasks := make([]*HookTask, 0, 10)
	err := x.Where("is_delivered=?", false).Find(&tasks)
	if err != nil {
		log.Error("DeliverHooks: %v", err)
		return
	}

	// Update hook task status.
	for _, t := range tasks {
		if err = t.deliver(); err != nil {
			log.Error("deliver: %v", err)
		}
	}

	// Start listening on new hook requests.
	for repoIDStr := range HookQueue.Queue() {
		log.Trace("DeliverHooks [repo_id: %v]", repoIDStr)
		HookQueue.Remove(repoIDStr)

		repoID, err := com.StrTo(repoIDStr).Int64()
		if err != nil {
			log.Error("Invalid repo ID: %s", repoIDStr)
			continue
		}

		tasks = make([]*HookTask, 0, 5)
		if err := x.Where("repo_id=? AND is_delivered=?", repoID, false).Find(&tasks); err != nil {
			log.Error("Get repository [%d] hook tasks: %v", repoID, err)
			continue
		}
		for _, t := range tasks {
			if err = t.deliver(); err != nil {
				log.Error("deliver: %v", err)
			}
		}
	}
}

var webhookHTTPClient *http.Client

// InitDeliverHooks starts the hooks delivery thread
func InitDeliverHooks() {
	timeout := time.Duration(setting.Webhook.DeliverTimeout) * time.Second

	webhookHTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Webhook.SkipTLSVerify},
			Proxy:           http.ProxyFromEnvironment,
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, timeout)
				if err != nil {
					return nil, err
				}

				return conn, conn.SetDeadline(time.Now().Add(timeout))

			},
		},
	}

	go DeliverHooks()
}
