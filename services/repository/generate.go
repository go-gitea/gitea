// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

// GenerateRepository generates a repository from a template
func GenerateRepository(doer, owner *models.User, templateRepo *models.Repository, opts models.GenerateRepoOptions) (*models.Repository, error) {
	generateRepo, err := models.GenerateRepository(doer, owner, templateRepo, opts)
	if err != nil {
		if generateRepo != nil {
			if errDelete := models.DeleteRepository(doer, owner.ID, generateRepo.ID); errDelete != nil {
				log.Error("Rollback deleteRepository: %v", errDelete)
			}
		}
		return nil, err
	}

	ctx, sess, err := models.TxDBContext()
	if err != nil {
		return generateRepo, err
	}

	// Git Content
	if opts.GitContent && !templateRepo.IsEmpty {
		if err := models.GenerateGitContent(ctx, templateRepo, generateRepo); err != nil {
			return generateRepo, err
		}
	}

	// Topics
	if opts.Topics {
		if err := models.GenerateTopics(ctx, templateRepo, generateRepo); err != nil {
			return generateRepo, err
		}
	}

	// Git Hooks
	if opts.GitHooks {
		if err := models.GenerateGitHooks(ctx, templateRepo, generateRepo); err != nil {
			return generateRepo, err
		}
	}

	// Webhooks
	if opts.Webhooks {
		if err := models.GenerateWebhooks(ctx, templateRepo, generateRepo); err != nil {
			return generateRepo, err
		}
	}

	notification.NotifyCreateRepository(doer, owner, generateRepo)

	return generateRepo, sess.Commit()
}
