// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
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

	"github.com/gobwas/glob"
)

type webhook struct {
	name           webhook_model.HookType
	payloadCreator func(p api.Payloader, event webhook_model.HookEventType, meta string) (api.Payloader, error)
}

var webhooks = map[webhook_model.HookType]*webhook{
	webhook_model.SLACK: {
		name:           webhook_model.SLACK,
		payloadCreator: GetSlackPayload,
	},
	webhook_model.DISCORD: {
		name:           webhook_model.DISCORD,
		payloadCreator: GetDiscordPayload,
	},
	webhook_model.DINGTALK: {
		name:           webhook_model.DINGTALK,
		payloadCreator: GetDingtalkPayload,
	},
	webhook_model.TELEGRAM: {
		name:           webhook_model.TELEGRAM,
		payloadCreator: GetTelegramPayload,
	},
	webhook_model.MSTEAMS: {
		name:           webhook_model.MSTEAMS,
		payloadCreator: GetMSTeamsPayload,
	},
	webhook_model.FEISHU: {
		name:           webhook_model.FEISHU,
		payloadCreator: GetFeishuPayload,
	},
	webhook_model.MATRIX: {
		name:           webhook_model.MATRIX,
		payloadCreator: GetMatrixPayload,
	},
	webhook_model.WECHATWORK: {
		name:           webhook_model.WECHATWORK,
		payloadCreator: GetWechatworkPayload,
	},
	webhook_model.PACKAGIST: {
		name:           webhook_model.PACKAGIST,
		payloadCreator: GetPackagistPayload,
	},
}

// RegisterWebhook registers a webhook
func RegisterWebhook(name string, webhook *webhook) {
	webhooks[name] = webhook
}

// IsValidHookTaskType returns true if a webhook registered
func IsValidHookTaskType(name string) bool {
	if name == webhook_model.GITEA || name == webhook_model.GOGS {
		return true
	}
	_, ok := webhooks[name]
	return ok
}

// hookQueue is a global queue of web hooks
var hookQueue queue.UniqueQueue

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
func handle(data ...queue.Data) []queue.Data {
	ctx := graceful.GetManager().HammerContext()

	for _, taskID := range data {
		task, err := webhook_model.GetHookTaskByID(ctx, taskID.(int64))
		if err != nil {
			log.Error("GetHookTaskByID[%d] failed: %v", taskID.(int64), err)
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
func PrepareWebhook(ctx context.Context, w *webhook_model.Webhook, event webhook_model.HookEventType, p api.Payloader) error {
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
		w.Type != webhook_model.GITEA && w.Type != webhook_model.GOGS &&
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
func PrepareWebhooks(ctx context.Context, source EventSource, event webhook_model.HookEventType, p api.Payloader) error {
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

	// check if owner is an org and append additional webhooks
	if owner != nil && owner.IsOrganization() {
		orgHooks, err := webhook_model.ListWebhooksByOpts(ctx, &webhook_model.ListWebhookOptions{
			OrgID:    owner.ID,
			IsActive: util.OptionalBoolTrue,
		})
		if err != nil {
			return fmt.Errorf("ListWebhooksByOpts: %w", err)
		}
		ws = append(ws, orgHooks...)
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
