// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/gobwas/glob"
)

type webhook struct {
	name           webhook_module.HookType
	payloadCreator func(p api.Payloader, event webhook_module.HookEventType, meta string) (api.Payloader, error)
}

var webhooks = map[webhook_module.HookType]*webhook{
	webhook_module.SLACK: {
		name:           webhook_module.SLACK,
		payloadCreator: GetSlackPayload,
	},
	webhook_module.DISCORD: {
		name:           webhook_module.DISCORD,
		payloadCreator: GetDiscordPayload,
	},
	webhook_module.DINGTALK: {
		name:           webhook_module.DINGTALK,
		payloadCreator: GetDingtalkPayload,
	},
	webhook_module.TELEGRAM: {
		name:           webhook_module.TELEGRAM,
		payloadCreator: GetTelegramPayload,
	},
	webhook_module.MSTEAMS: {
		name:           webhook_module.MSTEAMS,
		payloadCreator: GetMSTeamsPayload,
	},
	webhook_module.FEISHU: {
		name:           webhook_module.FEISHU,
		payloadCreator: GetFeishuPayload,
	},
	webhook_module.MATRIX: {
		name:           webhook_module.MATRIX,
		payloadCreator: GetMatrixPayload,
	},
	webhook_module.WECHATWORK: {
		name:           webhook_module.WECHATWORK,
		payloadCreator: GetWechatworkPayload,
	},
	webhook_module.PACKAGIST: {
		name:           webhook_module.PACKAGIST,
		payloadCreator: GetPackagistPayload,
	},
}

// IsValidHookTaskType returns true if a webhook registered
func IsValidHookTaskType(name string) bool {
	if name == webhook_module.GITEA || name == webhook_module.GOGS {
		return true
	}
	_, ok := webhooks[name]
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

// PrepareWebhook creates a hook task and enqueues it for processing
func PrepareWebhook(ctx context.Context, w *webhook_model.Webhook, event webhook_module.HookEventType, p api.Payloader) error {
	// Skip sending if webhooks are disabled.
	if setting.DisableWebhooks {
		return nil
	}

	for _, e := range w.EventCheckers() {
		if event == e.Type {
			if !e.Has() {
				return nil
			}

			break
		}
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

	var payloader api.Payloader
	var err error
	webhook, ok := webhooks[w.Type]
	if ok {
		payloader, err = webhook.payloadCreator(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("create payload for %s[%s]: %w", w.Type, event, err)
		}
	} else {
		payloader = p
	}

	task, err := webhook_model.CreateHookTask(ctx, &webhook_model.HookTask{
		HookID:    w.ID,
		Payloader: payloader,
		EventType: event,
	})
	if err != nil {
		return fmt.Errorf("CreateHookTask: %w", err)
	}

	return enqueueHookTask(task.ID)
}

// PrepareWebhooks adds new webhooks to task queue for given payload.
func PrepareWebhooks(ctx context.Context, source EventSource, event webhook_module.HookEventType, p api.Payloader) error {
	owner := source.Owner

	var ws []*webhook_model.Webhook

	if source.Repository != nil {
		repoHooks, err := webhook_model.ListWebhooksByOpts(ctx, &webhook_model.ListWebhookOptions{
			RepoID:   source.Repository.ID,
			IsActive: util.OptionalBoolTrue,
		})
		if err != nil {
			return fmt.Errorf("ListWebhooksByOpts: %w", err)
		}
		ws = append(ws, repoHooks...)

		owner = source.Repository.MustOwner(ctx)
	}

	// append additional webhooks of a user or organization
	if owner != nil {
		ownerHooks, err := webhook_model.ListWebhooksByOpts(ctx, &webhook_model.ListWebhookOptions{
			OwnerID:  owner.ID,
			IsActive: util.OptionalBoolTrue,
		})
		if err != nil {
			return fmt.Errorf("ListWebhooksByOpts: %w", err)
		}
		ws = append(ws, ownerHooks...)
	}

	// Add any admin-defined system webhooks
	systemHooks, err := webhook_model.GetSystemWebhooks(ctx, util.OptionalBoolTrue)
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
