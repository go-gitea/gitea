// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"

	"code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"

	ap "github.com/go-ap/activitypub"
)

// Create a comment
func Comment(ctx context.Context, note *ap.Note) error {
	actorUser, err := PersonIRIToUser(ctx, note.AttributedTo.GetLink())
	if err != nil {
		return err
	}

	username, reponame, idx, err := TicketIRIToName(note.Context.GetLink())
	if err != nil {
		return err
	}
	repo, err := repo_model.GetRepositoryByOwnerAndNameCtx(ctx, username, reponame)
	if err != nil {
		return err
	}
	issue, err := issues.GetIssueByIndex(repo.ID, idx)
	if err != nil {
		return err
	}
	_, err = issues.CreateCommentCtx(ctx, &issues.CreateCommentOptions{
		Doer:    actorUser,
		Repo:    repo,
		Issue:   issue,
		Content: note.Content.String(),
	})
	return err
}
