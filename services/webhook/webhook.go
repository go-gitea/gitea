// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/gobwas/glob"
)

type Requester func(context.Context, *webhook_model.Webhook, *webhook_model.HookTask) (req *http.Request, body []byte, err error)

var webhookRequesters = map[webhook_module.HookType]Requester{}

func RegisterWebhookRequester(hookType webhook_module.HookType, requester Requester) {
	webhookRequesters[hookType] = requester
}

// IsValidHookTaskType returns true if a webhook registered
func IsValidHookTaskType(name string) bool {
	if name == webhook_module.GITEA || name == webhook_module.GOGS {
		return true
	}
	_, ok := webhookRequesters[name]
	return ok
}

// hookQueue is a global queue of web hooks
var hookQueue *queue.WorkerPoolQueue[int64]

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

// EventSource represents the source of a webhook action. Repository and/or Owner must be set.
type EventSource struct {
	Repository *repo_model.Repository
	Owner      *user_model.User
}

// handle delivers hook tasks
func handler(items ...int64) []int64 {
	ctx := graceful.GetManager().HammerContext()

	for _, taskID := range items {
		task, err := webhook_model.GetHookTaskByID(ctx, taskID)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				log.Warn("GetHookTaskByID[%d] warn: %v", taskID, err)
			} else {
				log.Error("GetHookTaskByID[%d] failed: %v", taskID, err)
			}
			continue
		}

		if task.IsDelivered {
			// Already delivered in the meantime
			log.Trace("Task[%d] has already been delivered", task.ID)
			continue
		}

		if err := Deliver(ctx, task); err != nil {
			log.Error("Unable to deliver webhook task[%d]: %v", task.ID, err)
		}
	}

	return nil
}

func enqueueHookTask(taskID int64) error {
	err := hookQueue.Push(taskID)
	if err != nil && err != queue.ErrAlreadyInQueue {
		return err
	}
	return nil
}

func checkBranch(w *webhook_model.Webhook, branch string) bool {
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

// PrepareWebhook creates a hook task and enqueues it for processing.
// The payload is saved as-is. The adjustments depending on the webhook type happen
// right before delivery, in the [Deliver] method.
func PrepareWebhook(ctx context.Context, w *webhook_model.Webhook, event webhook_module.HookEventType, p api.Payloader) error {
	// Skip sending if webhooks are disabled.
	if setting.DisableWebhooks {
		return nil
	}

	if !w.HasEvent(event) {
		return nil
	}

	// Avoid sending "0 new commits" to non-integration relevant webhooks (e.g. slack, discord, etc.).
	// Integration webhooks (e.g. drone) still receive the required data.
	if pushEvent, ok := p.(*api.PushPayload); ok &&
		w.Type != webhook_module.GITEA && w.Type != webhook_module.GOGS &&
		len(pushEvent.Commits) == 0 {
		return nil
	}

	// If payload has no associated branch (e.g. it's a new tag, issue, etc.),
	// branch filter has no effect.
	if branch := getPayloadBranch(p); branch != "" {
		if !checkBranch(w, branch) {
			log.Info("Branch %q doesn't match branch filter %q, skipping", branch, w.BranchFilter)
			return nil
		}
	}

	payload, err := p.JSONPayload()
	if err != nil {
		return fmt.Errorf("JSONPayload for %s: %w", event, err)
	}

	task, err := webhook_model.CreateHookTask(ctx, &webhook_model.HookTask{
		HookID:         w.ID,
		PayloadContent: string(payload),
		EventType:      event,
		PayloadVersion: 2,
	})
	if err != nil {
		return fmt.Errorf("CreateHookTask for %s: %w", event, err)
	}

	return enqueueHookTask(task.ID)
}

// PrepareWebhooks adds new webhooks to task queue for given payload.
func PrepareWebhooks(ctx context.Context, source EventSource, event webhook_module.HookEventType, p api.Payloader) error {
	owner := source.Owner

	var ws []*webhook_model.Webhook

	if source.Repository != nil {
		repoHooks, err := db.Find[webhook_model.Webhook](ctx, webhook_model.ListWebhookOptions{
			RepoID:   source.Repository.ID,
			IsActive: optional.Some(true),
		})
		if err != nil {
			return fmt.Errorf("ListWebhooksByOpts: %w", err)
		}
		ws = append(ws, repoHooks...)

		owner = source.Repository.MustOwner(ctx)
	}

	// append additional webhooks of a user or organization
	if owner != nil {
		ownerHooks, err := db.Find[webhook_model.Webhook](ctx, webhook_model.ListWebhookOptions{
			OwnerID:  owner.ID,
			IsActive: optional.Some(true),
		})
		if err != nil {
			return fmt.Errorf("ListWebhooksByOpts: %w", err)
		}
		ws = append(ws, ownerHooks...)
	}

	// Add any admin-defined system webhooks
	systemHooks, err := webhook_model.GetSystemWebhooks(ctx, optional.Some(true))
	if err != nil {
		return fmt.Errorf("GetSystemWebhooks: %w", err)
	}
	ws = append(ws, systemHooks...)

	if len(ws) == 0 {
		return nil
	}

	for _, w := range ws {
		if err := PrepareWebhook(ctx, w, event, p); err != nil {
			return err
		}
	}
	return nil
}

// ReplayHookTask replays a webhook task
func ReplayHookTask(ctx context.Context, w *webhook_model.Webhook, uuid string) error {
	task, err := webhook_model.ReplayHookTask(ctx, w.ID, uuid)
	if err != nil {
		return err
	}

	return enqueueHookTask(task.ID)
}
