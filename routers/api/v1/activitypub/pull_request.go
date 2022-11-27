// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/activitypub"
	pull_service "code.gitea.io/gitea/services/pull"
)

func createPullRequest(ctx context.Context, ticket *forgefed.Ticket) error {
	// TODO: Clean this up

	actorUser, err := activitypub.PersonIRIToUser(ctx, ticket.AttributedTo.GetLink())
	if err != nil {
		log.Warn("Couldn't find ticket actor user", err)
	}

	// TODO: The IRI processing stuff should be moved to iri.go
	originIRI := ticket.Origin.GetLink()
	originIRISplit := strings.Split(originIRI.String(), "/")
	originInstance := originIRISplit[2]
	originUsername := originIRISplit[3]
	originReponame := originIRISplit[4]
	originBranch := originIRISplit[len(originIRISplit)-1]
	originRepo, _ := repo_model.GetRepositoryByOwnerAndName(originUsername+"@"+originInstance, originReponame)

	targetIRI := ticket.Target.GetLink()
	targetIRISplit := strings.Split(targetIRI.String(), "/")
	// targetInstance := targetIRISplit[2]
	targetUsername := targetIRISplit[3]
	targetReponame := targetIRISplit[4]
	targetBranch := targetIRISplit[len(targetIRISplit)-1]

	targetRepo, _ := repo_model.GetRepositoryByOwnerAndName(targetUsername, targetReponame)

	prIssue := &issues_model.Issue{
		RepoID:   targetRepo.ID,
		Title:    "Hello from test.exozy.me!", // Don't hardcode, get the title from the Ticket object
		PosterID: actorUser.ID,
		Poster:   actorUser,
		IsPull:   true,
		Content:  "🎉", // TODO: Get content from Ticket object
	}

	pr := &issues_model.PullRequest{
		HeadRepoID: originRepo.ID,
		BaseRepoID: targetRepo.ID,
		HeadBranch: originBranch,
		BaseBranch: targetBranch,
		HeadRepo:   originRepo,
		BaseRepo:   targetRepo,
		MergeBase:  "",
		Type:       issues_model.PullRequestGitea,
	}

	return pull_service.NewPullRequest(ctx, targetRepo, prIssue, []int64{}, []string{}, pr, []int64{})
}
