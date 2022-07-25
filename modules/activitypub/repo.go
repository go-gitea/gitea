// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	//"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/forgefed"
	/*repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/repository"

	ap "github.com/go-ap/activitypub"*/
)

func FederatedRepoNew(repo forgefed.Repository) error {
	// TODO: also handle forks
	/*_, err := repository.CreateRepository(user, user, models.CreateRepoOptions{
		Name: repo.Name.String(),
	})*/
	return nil
}
