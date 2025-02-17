// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package incoming

import (
	"context"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	incoming_payload "code.gitea.io/gitea/services/mailer/incoming/payload"
	"code.gitea.io/gitea/services/mailer/token"
)

func GenerateMailToRepoURL(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, event user_model.RepositoryRandsType) (string, string, error) {
	_, err := doer.GetOrCreateRandsForRepository(ctx, repo.ID, event)
	if err != nil {
		return "", "", err
	}

	payload, err := incoming_payload.CreateReferencePayload(&incoming_payload.ReferenceRepository{
		RepositoryID: repo.ID,
		ActionType:   incoming_payload.ReferenceRepositoryActionTypeNewIssue,
	})
	if err != nil {
		return "", "", err
	}

	token, err := token.CreateToken(ctx, token.NewIssueHandlerType, doer, payload)
	if err != nil {
		return "", "", err
	}

	mailToAddress := strings.Replace(setting.IncomingEmail.ReplyToAddress, setting.IncomingEmail.TokenPlaceholder, token, 1)
	return token, mailToAddress, nil
}
