// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"

	ap "github.com/go-ap/activitypub"
)

// Create a comment
func Comment(ctx context.Context, note ap.Note) {
	actorUser, err := personIRIToUser(ctx, note.AttributedTo.GetLink())
	if err != nil {
		log.Warn("Couldn't find actor", err)
		return
	}

	// TODO: Move IRI processing stuff to iri.go
	context := note.Context.GetLink()
	contextSplit := strings.Split(context.String(), "/")
	username := contextSplit[3]
	reponame := contextSplit[4]
	repo, _ := repo_model.GetRepositoryByOwnerAndName(username, reponame)

	idx, _ := strconv.ParseInt(contextSplit[len(contextSplit)-1], 10, 64)
	issue, _ := issues.GetIssueByIndex(repo.ID, idx)
	issues.CreateCommentCtx(ctx, &issues.CreateCommentOptions{
		Doer:    actorUser,
		Repo:    repo,
		Issue:   issue,
		Content: note.Content.String(),
	})
}
