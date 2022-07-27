// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

// Create a new federated user from a Person object
func FederatedUserNew(ctx context.Context, person ap.Person) error {
	name, err := personIRIToName(person.GetLink())
	if err != nil {
		return err
	}

	exists, err := user_model.IsUserExist(ctx, 0, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var email string
	if person.Location != nil {
		email = person.Location.GetLink().String()
	} else {
		// This might not even work
		email = strings.ReplaceAll(name, "@", "+") + "@" + setting.Service.NoReplyAddress
	}

	var avatar string
	if person.Icon != nil {
		icon := person.Icon.(*ap.Image)
		// Currently doesn't work
		avatar = icon.URL.GetLink().String()
	} else {
		avatar = ""
	}

	user := &user_model.User{
		Name:      name,
		FullName:  person.Name.String(), // May not exist!!
		Email:     email,
		Avatar:    avatar,
		LoginType: auth.Federated,
		LoginName: person.GetLink().String(),
	}
	return user_model.CreateUser(user)
}
