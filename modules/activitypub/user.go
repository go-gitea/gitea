// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"

	ap "github.com/go-ap/activitypub"
)

func FederatedUserNew(IRI ap.IRI) error {
	name, err := personIRIToName(IRI)
	if err != nil {
		return err
	}

	user := &user_model.User{
		Name:      name,
		Email:     name, // TODO: change this to something else to prevent collisions with normal users, maybe fetch email using Gitea API
		LoginType: auth.Federated,
		Website:   IRI.String(),
	}
	return user_model.CreateUser(user)
}
