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

func Follow(ctx context.Context, activity ap.Follow) {
	actorIRI := activity.Actor.GetID()
	objectIRI := activity.Object.GetID()
	actorIRISplit := strings.Split(actorIRI.String(), "/")
	objectIRISplit := strings.Split(objectIRI.String(), "/")
	actorName := actorIRISplit[len(actorIRISplit)-1] + "@" + actorIRISplit[2]
	objectName := objectIRISplit[len(objectIRISplit)-1]

	log.Warn("Follow object", activity.Object)

	err := FederatedUserNew(actorName, actorIRI)
	if err != nil {
		log.Warn("Couldn't create new user", err)
	}
	actorUser, err := user_model.GetUserByName(ctx, actorName)
	if err != nil {
		log.Warn("Couldn't find actor", err)
	}
	objectUser, err := user_model.GetUserByName(ctx, objectName)
	if err != nil {
		log.Warn("Couldn't find object", err)
	}

	user_model.FollowUser(actorUser.ID, objectUser.ID)

	accept := ap.AcceptNew(objectIRI, activity)
	accept.Actor = ap.Person{ID: objectIRI}
	accept.To = ap.ItemCollection{ap.IRI(actorIRI.String() + "/inbox")}

	Send(objectUser, accept)
}
