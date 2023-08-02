// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	gouuid "github.com/google/uuid"
)

//   ___ ___                __   ___________              __
//  /   |   \  ____   ____ |  | _\__    ___/____    _____|  | __
// /    ~    \/  _ \ /  _ \|  |/ / |    |  \__  \  /  ___/  |/ /
// \    Y    (  <_> |  <_> )    <  |    |   / __ \_\___ \|    <
//  \___|_  / \____/ \____/|__|_ \ |____|  (____  /____  >__|_ \
//        \/                    \/              \/     \/     \/

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
	ID             int64  `xorm:"pk autoincr"`
	HookID         int64  `xorm:"index"`
	UUID           string `xorm:"unique"`
	api.Payloader  `xorm:"-"`
	PayloadContent string `xorm:"LONGTEXT"`
	EventType      webhook_module.HookEventType
	IsDelivered    bool
	Delivered      timeutil.TimeStampNano

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

func (t *HookTask) simpleMarshalJSON(v any) string {
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
	t.UUID = gouuid.New().String()
	if t.Payloader != nil {
		data, err := t.Payloader.JSONPayload()
		if err != nil {
			return nil, err
		}
		t.PayloadContent = string(data)
	}
	if t.Delivered == 0 {
		t.Delivered = timeutil.TimeStampNanoNow()
	}
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

	return CreateHookTask(ctx, &HookTask{
		HookID:         task.HookID,
		PayloadContent: task.PayloadContent,
		EventType:      task.EventType,
	})
}

// FindUndeliveredHookTaskIDs will find the next 100 undelivered hook tasks with ID greater than the provided lowerID
func FindUndeliveredHookTaskIDs(ctx context.Context, lowerID int64) ([]int64, error) {
	const batchSize = 100

	tasks := make([]int64, 0, batchSize)
	return tasks, db.GetEngine(ctx).
		Select("id").
		Table(new(HookTask)).
		Where("is_delivered=?", false).
		And("id > ?", lowerID).
		Asc("id").
		Limit(batchSize).
		Find(&tasks)
}

func MarkTaskDelivered(ctx context.Context, task *HookTask) (bool, error) {
	count, err := db.GetEngine(ctx).ID(task.ID).Where("is_delivered = ?", false).Cols("is_delivered").Update(&HookTask{
		ID:          task.ID,
		IsDelivered: true,
	})

	return count != 0, err
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
			if err = deleteDeliveredHookTasksByWebhook(ctx, hookID, numberToKeep); err != nil {
				return err
			}
		}
	}
	log.Trace("Finished: CleanupHookTaskTable")
	return nil
}

func deleteDeliveredHookTasksByWebhook(ctx context.Context, hookID int64, numberDeliveriesToKeep int) error {
	log.Trace("Deleting hook_task rows for webhook %d, keeping the most recent %d deliveries", hookID, numberDeliveriesToKeep)
	deliveryDates := make([]int64, 0, 10)
	err := db.GetEngine(ctx).Table("hook_task").
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
		deletes, err := db.GetEngine(ctx).
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
