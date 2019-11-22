// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	"github.com/unknwon/com"
)

// generateRepository initializes repository from template
func generateRepository(e Engine, repo, templateRepo *Repository) (err error) {
	tmpDir := filepath.Join(os.TempDir(), "gitea-"+repo.Name+"-"+com.ToStr(time.Now().Nanosecond()))

	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", tmpDir, err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Error("RemoveAll: %v", err)
		}
	}()

	if err = generateRepoCommit(e, repo, templateRepo, tmpDir); err != nil {
		return fmt.Errorf("generateRepoCommit: %v", err)
	}

	// re-fetch repo
	if repo, err = getRepositoryByID(e, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %v", err)
	}

	repo.DefaultBranch = "master"
	if err = updateRepository(e, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return nil
}

// GenerateRepository generates a repository from a template
func GenerateRepository(ctx DBContext, doer, owner *User, templateRepo *Repository, opts GenerateRepoOptions) (_ *Repository, err error) {
	generateRepo := &Repository{
		OwnerID:       owner.ID,
		Owner:         owner,
		Name:          opts.Name,
		LowerName:     strings.ToLower(opts.Name),
		Description:   opts.Description,
		IsPrivate:     opts.Private,
		IsEmpty:       !opts.GitContent || templateRepo.IsEmpty,
		IsFsckEnabled: templateRepo.IsFsckEnabled,
		TemplateID:    templateRepo.ID,
	}

	if err = createRepository(ctx.e, doer, owner, generateRepo); err != nil {
		return nil, err
	}

	repoPath := RepoPath(owner.Name, generateRepo.Name)
	if err = checkInitRepository(repoPath); err != nil {
		return generateRepo, err
	}

	return generateRepo, nil
}

// GenerateGitContent generates git content from a template repository
func GenerateGitContent(ctx DBContext, templateRepo, generateRepo *Repository) error {
	if err := generateRepository(ctx.e, generateRepo, templateRepo); err != nil {
		return err
	}

	if err := generateRepo.updateSize(ctx.e); err != nil {
		return fmt.Errorf("failed to update size for repository: %v", err)
	}

	if err := copyLFS(ctx.e, generateRepo, templateRepo); err != nil {
		return fmt.Errorf("failed to copy LFS: %v", err)
	}
	return nil
}

// GenerateTopics generates topics from a template repository
func GenerateTopics(ctx DBContext, templateRepo, generateRepo *Repository) error {
	for _, topic := range templateRepo.Topics {
		if _, err := addTopicByNameToRepo(ctx.e, generateRepo.ID, topic); err != nil {
			return err
		}
	}
	return nil
}

// GenerateGitHooks generates git hooks from a template repository
func GenerateGitHooks(ctx DBContext, templateRepo, generateRepo *Repository) error {
	generateGitRepo, err := git.OpenRepository(generateRepo.repoPath(ctx.e))
	if err != nil {
		return err
	}
	defer generateGitRepo.Close()

	templateGitRepo, err := git.OpenRepository(templateRepo.repoPath(ctx.e))
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
func GenerateWebhooks(ctx DBContext, templateRepo, generateRepo *Repository) error {
	templateWebhooks, err := GetWebhooksByRepoID(templateRepo.ID)
	if err != nil {
		return err
	}

	for _, templateWebhook := range templateWebhooks {
		generateWebhook := &Webhook{
			RepoID:       generateRepo.ID,
			URL:          templateWebhook.URL,
			HTTPMethod:   templateWebhook.HTTPMethod,
			ContentType:  templateWebhook.ContentType,
			Secret:       templateWebhook.Secret,
			HookEvent:    templateWebhook.HookEvent,
			IsActive:     templateWebhook.IsActive,
			HookTaskType: templateWebhook.HookTaskType,
			OrgID:        templateWebhook.OrgID,
			Events:       templateWebhook.Events,
			Meta:         templateWebhook.Meta,
		}
		if err := createWebhook(ctx.e, generateWebhook); err != nil {
			return err
		}
	}
	return nil
}
