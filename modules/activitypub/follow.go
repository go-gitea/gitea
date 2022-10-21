// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"strings"

	user_model "code.gitea.io/gitea/models/user"

	ap "github.com/go-ap/activitypub"
)

// Process a Follow activity
func Follow(ctx context.Context, follow ap.Follow) error {
	// Actor is the user performing the follow
	actorIRI := follow.Actor.GetLink()
	actorUser, err := PersonIRIToUser(ctx, actorIRI)
	if err != nil {
		return err
	}

	// Object is the user being followed
	objectIRI := follow.Object.GetLink()
	objectUser, err := PersonIRIToUser(ctx, objectIRI)
	// Must be a local user
	if err != nil || strings.Contains(objectUser.Name, "@") {
		return err
	}

	err = user_model.FollowUser(actorUser.ID, objectUser.ID)
	if err != nil {
		return err
	}

	// Send back an Accept activity
	accept := ap.AcceptNew(objectIRI, follow)
	accept.Actor = ap.Person{ID: objectIRI}
	accept.To = ap.ItemCollection{ap.IRI(actorIRI.String() + "/inbox")}
	accept.Object = follow
	return Send(objectUser, accept)
}

// Process a Undo follow activity
func Unfollow(ctx context.Context, unfollow ap.Undo) error {
	follow := unfollow.Object.(*ap.Follow)
	// Actor is the user performing the undo follow
	actorIRI := follow.Actor.GetLink()
	actorUser, err := PersonIRIToUser(ctx, actorIRI)
	if err != nil {
		return err
	}

	// Object is the user being unfollowed
	objectIRI := follow.Object.GetLink()
	objectUser, err := PersonIRIToUser(ctx, objectIRI)
	// Must be a local user
	if err != nil || strings.Contains(objectUser.Name, "@") {
		return err
	}

	return user_model.UnfollowUser(actorUser.ID, objectUser.ID)
}
