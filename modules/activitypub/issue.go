// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	ap "github.com/go-ap/activitypub"
)

func Comment(ctx context.Context, activity ap.Note) {
	actorIRI := activity.AttributedTo.GetLink()
	actorIRISplit := strings.Split(actorIRI.String(), "/")
	actorName := actorIRISplit[len(actorIRISplit)-1] + "@" + actorIRISplit[2]
	err := FederatedUserNew(actorName, actorIRI)
	if err != nil {
		log.Warn("Couldn't create new user", err)
	}
	actorUser, err := user_model.GetUserByName(ctx, actorName)
	if err != nil {
		log.Warn("Couldn't find actor", err)
	}

	context := activity.Context.GetLink()
	contextSplit := strings.Split(context.String(), "/")
	username := contextSplit[3]
	reponame := contextSplit[4]
	fmt.Println(username)
	fmt.Println(reponame)
	repo, _ := repo_model.GetRepositoryByOwnerAndName(username, reponame)
	idx, _ := strconv.ParseInt(contextSplit[len(contextSplit)-1], 10, 64)
	issue, _ := issues.GetIssueByIndex(repo.ID, idx)
	issues.CreateCommentCtx(ctx, &issues.CreateCommentOptions{
		Doer: actorUser,
		Repo: repo,
		Issue: issue,
		Content: activity.Content.String(),
	})
}
