// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/services/activitypub"
	repo_service "code.gitea.io/gitea/services/repository"

	ap "github.com/go-ap/activitypub"
)

func fork(ctx context.Context, create ap.Create) error {
	// Object is the new fork repository
	repository, err := ap.To[forgefed.Repository](create.Object)
	if err != nil {
		return nil
	}

	// TODO: Clean this up
	actor, err := activitypub.PersonIRIToUser(ctx, create.Actor.GetLink())
	if err != nil {
		return err
	}

	// Don't create an actual copy of the remote repo!
	// https://gitea.com/xy/gitea/issues/7

	// Create the fork
	repoIRI := repository.GetLink()
	username, reponame, err := activitypub.RepositoryIRIToName(repoIRI)
	if err != nil {
		return err
	}

	// FederatedUserNew(username + "@" + instance, )
	user, _ := user_model.GetUserByName(ctx, username)

	// var repo forgefed.Repository
	// repo = activity.Object
	repo, _ := repo_model.GetRepositoryByOwnerAndName(actor.Name, reponame) // hardcoded for now :(

	_, err = repo_service.ForkRepository(ctx, user, user, repo_service.ForkRepoOptions{BaseRepo: repo, Name: reponame, Description: "this is a remote fork"})
	return err
}
