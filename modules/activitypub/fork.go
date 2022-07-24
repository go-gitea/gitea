// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/forgefed"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/migrations"
	repo_service "code.gitea.io/gitea/services/repository"

	ap "github.com/go-ap/activitypub"
)

func Fork(ctx context.Context, instance, username, reponame, destUsername string) error {
	// TODO: Clean this up

	// Migrate repository code
	user, _ := user_model.GetUserByName(ctx, destUsername)
	_, err := migrations.MigrateRepository(ctx, user, destUsername, migrations.MigrateOptions{
		CloneAddr: "https://" + instance + "/" + username + "/" + reponame + ".git",
		RepoName:  reponame,
	}, nil)
	if err != nil {
		log.Warn("Couldn't create fork", err)
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

func ForkFromCreate(ctx context.Context, repository forgefed.Repository) error {
	// TODO: Clean this up

	// Don't create an actual copy of the remote repo!
	// https://gitea.com/Ta180m/gitea/issues/7

	// Create the fork
	repoIRI := repository.GetID()
	repoIRISplit := strings.Split(repoIRI.String(), "/")
	instance := repoIRISplit[2]
	username := repoIRISplit[7]
	reponame := repoIRISplit[8]

	// FederatedUserNew(username + "@" + instance, )
	user, _ := user_model.GetUserByName(ctx, username+"@"+instance)

	// var repo forgefed.Repository
	// repo = activity.Object
	repo, _ := repo_model.GetRepositoryByOwnerAndName("Ta180m", reponame) // hardcoded for now :(

	_, err := repo_service.ForkRepository(ctx, user, user, repo_service.ForkRepoOptions{BaseRepo: repo, Name: reponame, Description: "this is a remote fork"})
	return err
}
