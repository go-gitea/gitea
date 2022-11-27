// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
)

// Process an incoming Follow activity
func follow(ctx context.Context, follow ap.Follow) error {
	// Actor is the user performing the follow
	actorIRI := follow.Actor.GetLink()
	actorUser, err := activitypub.PersonIRIToUser(ctx, actorIRI)
	if err != nil {
		return err
	}

	// Object is the user being followed
	objectIRI := follow.Object.GetLink()
	objectUser, err := activitypub.PersonIRIToUser(ctx, objectIRI)
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
	return activitypub.Send(objectUser, accept)
}

// Process an incoming Undo follow activity
func unfollow(ctx context.Context, unfollow ap.Undo) error {
	// Object contains the follow
	follow, err := ap.To[ap.Follow](unfollow.Object)
	if err != nil {
		return err
	}

	// Actor is the user performing the undo follow
	actorIRI := follow.Actor.GetLink()
	actorUser, err := activitypub.PersonIRIToUser(ctx, actorIRI)
	if err != nil {
		return err
	}

	// Object is the user being unfollowed
	objectIRI := follow.Object.GetLink()
	objectUser, err := activitypub.PersonIRIToUser(ctx, objectIRI)
	// Must be a local user
	if err != nil || strings.Contains(objectUser.Name, "@") {
		return err
	}

	return user_model.UnfollowUser(actorUser.ID, objectUser.ID)
}
