// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/migrations"
	repo_service "code.gitea.io/gitea/services/repository"

	ap "github.com/go-ap/activitypub"
)

func CreateFork(ctx context.Context, instance, username, reponame, destUsername string) error {
	// TODO: Clean this up

	// Migrate repository code
	user, err := user_model.GetUserByName(ctx, destUsername)
	if err != nil {
		return err
	}

	_, err = migrations.MigrateRepository(ctx, user, destUsername, migrations.MigrateOptions{
		CloneAddr: "https://" + instance + "/" + username + "/" + reponame + ".git",
		RepoName:  reponame,
	}, nil)
	if err != nil {
		return err
	}

	// TODO: Make the migrated repo a fork

	// Send a Create activity to the instance we are forking from
	create := ap.Create{Type: ap.CreateType}
	create.To = ap.ItemCollection{ap.IRI("https://" + instance + "/api/v1/activitypub/repo/" + username + "/" + reponame + "/inbox")}
	repo := ap.IRI(setting.AppURL + "api/v1/activitypub/repo/" + destUsername + "/" + reponame)
	// repo := forgefed.RepositoryNew(ap.IRI(setting.AppURL + "api/v1/activitypub/repo/" + destUsername + "/" + reponame))
	// repo.ForkedFrom = forgefed.RepositoryNew(ap.IRI())
	create.Object = repo

	return Send(user, &create)
}

func ReceiveFork(ctx context.Context, create ap.Create) error {
	// TODO: Clean this up

	repository := create.Object.(*forgefed.Repository)

	actor, err := PersonIRIToUser(ctx, create.Actor.GetLink())
	if err != nil {
		return err
	}

	// Don't create an actual copy of the remote repo!
	// https://gitea.com/xy/gitea/issues/7

	// Create the fork
	repoIRI := repository.GetLink()
	username, reponame, err := RepositoryIRIToName(repoIRI)
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
