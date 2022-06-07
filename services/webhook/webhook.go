// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
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
	webhooks[webhook_model.HookType(name)] = webhook
}

// IsValidHookTaskType returns true if a webhook registered
func IsValidHookTaskType(name string) bool {
	if name == webhook_model.GITEA || name == webhook_model.GOGS {
		return true
	}
	_, ok := webhooks[webhook_model.HookType(name)]
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

// handle passed PR IDs and test the PRs
func handle(data ...queue.Data) []queue.Data {
	for _, datum := range data {
		repoIDStr := datum.(string)
		log.Trace("DeliverHooks [repo_id: %v]", repoIDStr)

		repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
		if err != nil {
			log.Error("Invalid repo ID: %s", repoIDStr)
			continue
		}

		tasks, err := webhook_model.FindRepoUndeliveredHookTasks(repoID)
		if err != nil {
			log.Error("Get repository [%d] hook tasks: %v", repoID, err)
			continue
		}
		for _, t := range tasks {
			if err = Deliver(graceful.GetManager().HammerContext(), t); err != nil {
				log.Error("deliver: %v", err)
			}
		}
	}
	return nil
}

func addToTask(repoID int64) error {
	err := hookQueue.PushFunc(strconv.FormatInt(repoID, 10), nil)
	if err != nil && err != queue.ErrAlreadyInQueue {
		return err
	}
	return nil
}

// PrepareWebhook adds special webhook to task queue for given payload.
func PrepareWebhook(w *webhook_model.Webhook, repo *repo_model.Repository, event webhook_model.HookEventType, p api.Payloader) error {
	if err := prepareWebhook(w, repo, event, p); err != nil {
		return err
	}

	return addToTask(repo.ID)
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

func prepareWebhook(w *webhook_model.Webhook, repo *repo_model.Repository, event webhook_model.HookEventType, p api.Payloader) error {
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
			return fmt.Errorf("create payload for %s[%s]: %v", w.Type, event, err)
		}
	} else {
		payloader = p
	}

	if err = webhook_model.CreateHookTask(&webhook_model.HookTask{
		RepoID:    repo.ID,
		HookID:    w.ID,
		Payloader: payloader,
		EventType: event,
	}); err != nil {
		return fmt.Errorf("CreateHookTask: %v", err)
	}
	return nil
}

// PrepareWebhooks adds new webhooks to task queue for given payload.
func PrepareWebhooks(repo *repo_model.Repository, event webhook_model.HookEventType, p api.Payloader) error {
	if err := prepareWebhooks(db.DefaultContext, repo, event, p); err != nil {
		return err
	}

	return addToTask(repo.ID)
}

func prepareWebhooks(ctx context.Context, repo *repo_model.Repository, event webhook_model.HookEventType, p api.Payloader) error {
	ws, err := webhook_model.ListWebhooksByOpts(ctx, &webhook_model.ListWebhookOptions{
		RepoID:   repo.ID,
		IsActive: util.OptionalBoolTrue,
	})
	if err != nil {
		return fmt.Errorf("GetActiveWebhooksByRepoID: %v", err)
	}

	// check if repo belongs to org and append additional webhooks
	if repo.MustOwner().IsOrganization() {
		// get hooks for org
		orgHooks, err := webhook_model.ListWebhooksByOpts(ctx, &webhook_model.ListWebhookOptions{
			OrgID:    repo.OwnerID,
			IsActive: util.OptionalBoolTrue,
		})
		if err != nil {
			return fmt.Errorf("GetActiveWebhooksByOrgID: %v", err)
		}
		ws = append(ws, orgHooks...)
	}

	// Add any admin-defined system webhooks
	systemHooks, err := webhook_model.GetSystemWebhooks(ctx, util.OptionalBoolTrue)
	if err != nil {
		return fmt.Errorf("GetSystemWebhooks: %v", err)
	}
	ws = append(ws, systemHooks...)

	if len(ws) == 0 {
		return nil
	}

	for _, w := range ws {
		if err = prepareWebhook(w, repo, event, p); err != nil {
			return err
		}
	}
	return nil
}

// ReplayHookTask replays a webhook task
func ReplayHookTask(w *webhook_model.Webhook, uuid string) error {
	t, err := webhook_model.ReplayHookTask(w.ID, uuid)
	if err != nil {
		return err
	}

	return addToTask(t.RepoID)
}
