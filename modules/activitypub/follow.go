// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	ap "github.com/go-ap/activitypub"
)

// Process a Follow activity
func Follow(ctx context.Context, follow ap.Follow) {
	// Actor is the user performing the follow
	actorIRI := follow.Actor.GetID()
	actorUser, err := personIRIToUser(ctx, actorIRI)
	if err != nil {
		log.Warn("Couldn't find actor user for follow", err)
		return
	}

	// Object is the user being followed
	objectIRI := follow.Object.GetID()
	objectUser, err := personIRIToUser(ctx, objectIRI)
	// Must be a local user
	if strings.Contains(objectUser.Name, "@") || err != nil {
		log.Warn("Couldn't find object user for follow", err)
		return
	}

	user_model.FollowUser(actorUser.ID, objectUser.ID)

	// Send back an Accept activity
	accept := ap.AcceptNew(objectIRI, follow)
	accept.Actor = ap.Person{ID: objectIRI}
	accept.To = ap.ItemCollection{ap.IRI(actorIRI.String() + "/inbox")}
	accept.Object = follow
	Send(objectUser, accept)
}

// Process a Undo follow activity
// I haven't tried this yet so hopefully it works
func Unfollow(ctx context.Context, unfollow ap.Undo) {
	follow := unfollow.Object.(*ap.Follow)
	// Actor is the user performing the undo follow
	actorIRI := follow.Actor.GetID()
	actorUser, err := personIRIToUser(ctx, actorIRI)
	if err != nil {
		log.Warn("Couldn't find actor user for follow", err)
		return
	}

	// Object is the user being unfollowed
	objectIRI := follow.Object.GetID()
	objectUser, err := personIRIToUser(ctx, objectIRI)
	// Must be a local user
	if strings.Contains(objectUser.Name, "@") || err != nil {
		log.Warn("Couldn't find object user for follow", err)
		return
	}

	user_model.UnfollowUser(actorUser.ID, objectUser.ID)
}
