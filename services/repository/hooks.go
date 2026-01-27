// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

// SyncRepositoryHooks rewrites all repositories' pre-receive, update and post-receive hooks
// to make sure the binary and custom conf path are up-to-date.
func SyncRepositoryHooks(ctx context.Context) error {
	log.Trace("Doing: SyncRepositoryHooks")

	if err := db.Iterate(
		ctx,
		builder.Gt{"id": 0},
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before sync repository hooks for %s", repo.FullName())
			default:
			}

			if err := gitrepo.CreateDelegateHooks(ctx, repo); err != nil {
				return fmt.Errorf("CreateDelegateHooks: %w", err)
			}

			if HasWiki(ctx, repo) {
				if err := gitrepo.CreateDelegateHooks(ctx, repo.WikiStorageRepo()); err != nil {
					return fmt.Errorf("CreateDelegateHooks: %w", err)
				}
			}
			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: SyncRepositoryHooks")
	return nil
}

// GenerateGitHooks generates git hooks from a template repository
func GenerateGitHooks(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) error {
	generateGitRepo, err := gitrepo.OpenRepository(ctx, generateRepo)
	if err != nil {
		return err
	}
	defer generateGitRepo.Close()

	templateGitRepo, err := gitrepo.OpenRepository(ctx, templateRepo)
	if err != nil {
		return err
	}
	defer templateGitRepo.Close()

	templateHooks, err := templateGitRepo.Hooks()
	if err != nil {
		return err
	}

	for _, templateHook := range templateHooks {
		generateHook, err := generateGitRepo.GetHook(templateHook.Name())
		if err != nil {
			return err
		}

		generateHook.Content = templateHook.Content
		if err := generateHook.Update(); err != nil {
			return err
		}
	}
	return nil
}

// GenerateWebhooks generates webhooks from a template repository
func GenerateWebhooks(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) error {
	templateWebhooks, err := db.Find[webhook.Webhook](ctx, webhook.ListWebhookOptions{RepoID: templateRepo.ID})
	if err != nil {
		return err
	}

	ws := make([]*webhook.Webhook, 0, len(templateWebhooks))
	for _, templateWebhook := range templateWebhooks {
		ws = append(ws, &webhook.Webhook{
			RepoID:      generateRepo.ID,
			URL:         templateWebhook.URL,
			HTTPMethod:  templateWebhook.HTTPMethod,
			ContentType: templateWebhook.ContentType,
			Secret:      templateWebhook.Secret,
			HookEvent:   templateWebhook.HookEvent,
			IsActive:    templateWebhook.IsActive,
			Type:        templateWebhook.Type,
			OwnerID:     templateWebhook.OwnerID,
			Events:      templateWebhook.Events,
			Meta:        templateWebhook.Meta,
		})
	}
	return webhook.CreateWebhooks(ctx, ws)
}
