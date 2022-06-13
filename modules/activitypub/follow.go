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

func Follow(ctx context.Context, activity ap.Follow) {
	actorIRI := activity.Actor.GetID()
	objectIRI := activity.Object.GetID()
	actorIRISplit := strings.Split(actorIRI.String(), "/")
	objectIRISplit := strings.Split(objectIRI.String(), "/")
	actorName := actorIRISplit[len(actorIRISplit)-1]
	objectName := objectIRISplit[len(actorIRISplit)-1]

	FederatedUserNew(actorName, actorIRI)
	actorUser, _ := user_model.GetUserByName(ctx, actorName)
	objectUser, _ := user_model.GetUserByName(ctx, objectName)
	
	user_model.FollowUser(actorUser.ID, objectUser.ID)

	accept := ap.AcceptNew(objectIRI, activity)
	accept.Actor = activity.Object
	accept.To = ap.ItemCollection{actorIRI}

	Send(objectUser, accept)
}
