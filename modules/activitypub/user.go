// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"strings"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/setting"
	user_model "code.gitea.io/gitea/models/user"

	ap "github.com/go-ap/activitypub"
)

func FederatedUserNew(person ap.Person) error {
	name, err := personIRIToName(person.GetLink())
	if err != nil {
		return err
	}

	var email string
	if person.Location != nil {
		email = person.Location.GetLink().String()
	} else {
		email = strings.ReplaceAll(name, "@", "+") + "@" + setting.Service.NoReplyAddress
	}

	var avatar string
	if person.Icon != nil {
		icon := person.Icon.(*ap.Image)
		avatar = icon.URL.GetLink().String()
	} else {
		avatar = ""
	}

	user := &user_model.User{
		Name:      name,
		FullName:  person.Name.String(),
		Email:     email,
		Avatar:    avatar,
		LoginType: auth.Federated,
		LoginName: person.GetLink().String(),
	}
	return user_model.CreateUser(user)
}
