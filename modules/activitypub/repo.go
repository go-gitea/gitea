// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/repository"

	ap "gitea.com/Ta180m/activitypub"
)

func FederatedRepoNew(user *user_model.User, name string, IRI ap.IRI) (*repo_model.Repository, error) {
	return repository.CreateRepository(user, user, models.CreateRepoOptions{
		Name: name,
	})
}
