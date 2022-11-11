// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

// FollowUser marks someone be another's follower.
func FollowUser(userID, followID int64) (err error) {
	if userID == followID || user_model.IsFollowing(userID, followID) {
		return nil
	}

	followUser, err := user_model.GetUserByID(followID)
	if err != nil {
		return err
	}
	if followUser.LoginType == auth.Federated {
		// Following remote user
		actorUser, err := user_model.GetUserByID(userID)
		if err != nil {
			return err
		}

		object := ap.PersonNew(ap.IRI(followUser.LoginName))
		follow := ap.FollowNew("", object)
		follow.Type = ap.FollowType
		follow.Actor = ap.PersonNew(ap.IRI(setting.AppURL + "api/v1/activitypub/user/" + actorUser.Name))
		follow.To = ap.ItemCollection{ap.Item(ap.IRI(followUser.LoginName + "/inbox"))}
		err = activitypub.Send(actorUser, follow)
		if err != nil {
			return err
		}
	}

	return user_model.FollowUser(userID, followID)
}

// UnfollowUser unmarks someone as another's follower.
func UnfollowUser(userID, followID int64) (err error) {
	if userID == followID || !user_model.IsFollowing(userID, followID) {
		return nil
	}

	return user_model.UnfollowUser(userID, followID)
}

// Create a new federated user from a Person object
func FederatedUserNew(ctx context.Context, person *ap.Person) error {
	name, err := activitypub.PersonIRIToName(person.GetLink())
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

	if person.PublicKey.PublicKeyPem == "" {
		return errors.New("person public key not found")
	}

	user := &user_model.User{
		Name:      name,
		FullName:  person.Name.String(), // May not exist!!
		Email:     email,
		LoginType: auth.Federated,
		LoginName: person.GetLink().String(),
	}
	err = user_model.CreateUser(user)
	if err != nil {
		return err
	}

	if person.Icon != nil {
		icon := person.Icon.(*ap.Image)
		iconURL, err := icon.URL.GetLink().URL()
		if err != nil {
			return err
		}

		body, err := activitypub.Fetch(iconURL)
		if err != nil {
			return err
		}

		err = UploadAvatar(user, body)
		if err != nil {
			return err
		}
	}

	err = user_model.SetUserSetting(user.ID, user_model.UserActivityPubPrivPem, "")
	if err != nil {
		return err
	}
	return user_model.SetUserSetting(user.ID, user_model.UserActivityPubPubPem, person.PublicKey.PublicKeyPem)
}
