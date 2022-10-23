// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	gouuid "github.com/google/uuid"
)

//   ___ ___                __   ___________              __
//  /   |   \  ____   ____ |  | _\__    ___/____    _____|  | __
// /    ~    \/  _ \ /  _ \|  |/ / |    |  \__  \  /  ___/  |/ /
// \    Y    (  <_> |  <_> )    <  |    |   / __ \_\___ \|    <
//  \___|_  / \____/ \____/|__|_ \ |____|  (____  /____  >__|_ \
//        \/                    \/              \/     \/     \/

// HookEventType is the type of an hook event
type HookEventType string

// Types of hook events
const (
	HookEventCreate                    HookEventType = "create"
	HookEventDelete                    HookEventType = "delete"
	HookEventFork                      HookEventType = "fork"
	HookEventPush                      HookEventType = "push"
	HookEventIssues                    HookEventType = "issues"
	HookEventIssueAssign               HookEventType = "issue_assign"
	HookEventIssueLabel                HookEventType = "issue_label"
	HookEventIssueMilestone            HookEventType = "issue_milestone"
	HookEventIssueComment              HookEventType = "issue_comment"
	HookEventPullRequest               HookEventType = "pull_request"
	HookEventPullRequestAssign         HookEventType = "pull_request_assign"
	HookEventPullRequestLabel          HookEventType = "pull_request_label"
	HookEventPullRequestMilestone      HookEventType = "pull_request_milestone"
	HookEventPullRequestComment        HookEventType = "pull_request_comment"
	HookEventPullRequestReviewApproved HookEventType = "pull_request_review_approved"
	HookEventPullRequestReviewRejected HookEventType = "pull_request_review_rejected"
	HookEventPullRequestReviewComment  HookEventType = "pull_request_review_comment"
	HookEventPullRequestSync           HookEventType = "pull_request_sync"
	HookEventWiki                      HookEventType = "wiki"
	HookEventRepository                HookEventType = "repository"
	HookEventRelease                   HookEventType = "release"
	HookEventPackage                   HookEventType = "package"
)

// Event returns the HookEventType as an event string
func (h HookEventType) Event() string {
	switch h {
	case HookEventCreate:
		return "create"
	case HookEventDelete:
		return "delete"
	case HookEventFork:
		return "fork"
	case HookEventPush:
		return "push"
	case HookEventIssues, HookEventIssueAssign, HookEventIssueLabel, HookEventIssueMilestone:
		return "issues"
	case HookEventPullRequest, HookEventPullRequestAssign, HookEventPullRequestLabel, HookEventPullRequestMilestone,
		HookEventPullRequestSync:
		return "pull_request"
	case HookEventIssueComment, HookEventPullRequestComment:
		return "issue_comment"
	case HookEventPullRequestReviewApproved:
		return "pull_request_approved"
	case HookEventPullRequestReviewRejected:
		return "pull_request_rejected"
	case HookEventPullRequestReviewComment:
		return "pull_request_comment"
	case HookEventWiki:
		return "wiki"
	case HookEventRepository:
		return "repository"
	case HookEventRelease:
		return "release"
	}
	return ""
}

// HookRequest represents hook task request information.
type HookRequest struct {
	URL        string            `json:"url"`
	HTTPMethod string            `json:"http_method"`
	Headers    map[string]string `json:"headers"`
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
	HookID          int64
	UUID            string
	api.Payloader   `xorm:"-"`
	PayloadContent  string `xorm:"LONGTEXT"`
	EventType       HookEventType
	IsDelivered     bool
	Delivered       int64
	DeliveredString string `xorm:"-"`

	// History info.
	IsSucceed       bool
	RequestContent  string        `xorm:"LONGTEXT"`
	RequestInfo     *HookRequest  `xorm:"-"`
	ResponseContent string        `xorm:"LONGTEXT"`
	ResponseInfo    *HookResponse `xorm:"-"`
}

func init() {
	db.RegisterModel(new(HookTask))
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
	return tasks, db.GetEngine(db.DefaultContext).
		Limit(setting.Webhook.PagingNum, (page-1)*setting.Webhook.PagingNum).
		Where("hook_id=?", hookID).
		Desc("id").
		Find(&tasks)
}

// CreateHookTask creates a new hook task,
// it handles conversion from Payload to PayloadContent.
func CreateHookTask(ctx context.Context, t *HookTask) (*HookTask, error) {
	data, err := t.Payloader.JSONPayload()
	if err != nil {
		return nil, err
	}
	t.UUID = gouuid.New().String()
	t.PayloadContent = string(data)
	return t, db.Insert(ctx, t)
}

func GetHookTaskByID(ctx context.Context, id int64) (*HookTask, error) {
	t := &HookTask{}

	has, err := db.GetEngine(ctx).ID(id).Get(t)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrHookTaskNotExist{
			TaskID: id,
		}
	}
	return t, nil
}

// UpdateHookTask updates information of hook task.
func UpdateHookTask(t *HookTask) error {
	_, err := db.GetEngine(db.DefaultContext).ID(t.ID).AllCols().Update(t)
	return err
}

// ReplayHookTask copies a hook task to get re-delivered
func ReplayHookTask(ctx context.Context, hookID int64, uuid string) (*HookTask, error) {
	task := &HookTask{
		HookID: hookID,
		UUID:   uuid,
	}
	has, err := db.GetByBean(ctx, task)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrHookTaskNotExist{
			HookID: hookID,
			UUID:   uuid,
		}
	}

	newTask := &HookTask{
		UUID:           gouuid.New().String(),
		HookID:         task.HookID,
		PayloadContent: task.PayloadContent,
		EventType:      task.EventType,
	}
	return newTask, db.Insert(ctx, newTask)
}

// FindUndeliveredHookTasks represents find the undelivered hook tasks
func FindUndeliveredHookTasks(ctx context.Context) ([]*HookTask, error) {
	tasks := make([]*HookTask, 0, 10)
	return tasks, db.GetEngine(ctx).
		Where("is_delivered=?", false).
		Find(&tasks)
}

// CleanupHookTaskTable deletes rows from hook_task as needed.
func CleanupHookTaskTable(ctx context.Context, cleanupType HookTaskCleanupType, olderThan time.Duration, numberToKeep int) error {
	log.Trace("Doing: CleanupHookTaskTable")

	if cleanupType == OlderThan {
		deleteOlderThan := time.Now().Add(-olderThan).UnixNano()
		deletes, err := db.GetEngine(ctx).
			Where("is_delivered = ? and delivered < ?", true, deleteOlderThan).
			Delete(new(HookTask))
		if err != nil {
			return err
		}
		log.Trace("Deleted %d rows from hook_task", deletes)
	} else if cleanupType == PerWebhook {
		hookIDs := make([]int64, 0, 10)
		err := db.GetEngine(ctx).
			Table("webhook").
			Where("id > 0").
			Cols("id").
			Find(&hookIDs)
		if err != nil {
			return err
		}
		for _, hookID := range hookIDs {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("Before deleting hook_task records for hook id %d", hookID)
			default:
			}
			if err = deleteDeliveredHookTasksByWebhook(hookID, numberToKeep); err != nil {
				return err
			}
		}
	}
	log.Trace("Finished: CleanupHookTaskTable")
	return nil
}

func deleteDeliveredHookTasksByWebhook(hookID int64, numberDeliveriesToKeep int) error {
	log.Trace("Deleting hook_task rows for webhook %d, keeping the most recent %d deliveries", hookID, numberDeliveriesToKeep)
	deliveryDates := make([]int64, 0, 10)
	err := db.GetEngine(db.DefaultContext).Table("hook_task").
		Where("hook_task.hook_id = ? AND hook_task.is_delivered = ? AND hook_task.delivered is not null", hookID, true).
		Cols("hook_task.delivered").
		Join("INNER", "webhook", "hook_task.hook_id = webhook.id").
		OrderBy("hook_task.delivered desc").
		Limit(1, numberDeliveriesToKeep).
		Find(&deliveryDates)
	if err != nil {
		return err
	}

	if len(deliveryDates) > 0 {
		deletes, err := db.GetEngine(db.DefaultContext).
			Where("hook_id = ? and is_delivered = ? and delivered <= ?", hookID, true, deliveryDates[0]).
			Delete(new(HookTask))
		if err != nil {
			return err
		}
		log.Trace("Deleted %d hook_task rows for webhook %d", deletes, hookID)
	} else {
		log.Trace("No hook_task rows to delete for webhook %d", hookID)
	}

	return nil
}
