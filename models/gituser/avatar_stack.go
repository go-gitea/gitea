// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gituser

import (
	"context"

	"codenest.dev/models/user"
	"codenest.dev/modules/container"
	"codenest.dev/modules/git"
	"codenest.dev/modules/log"
)

// AvatarStackData is the view-model for the AvatarStack render helpers. Participants[0] is
// the primary participant (commit author), painted on top; the rest follow.
type AvatarStackData struct {
	Participants      []*CommitParticipant
	SearchByEmailLink string
}

func BuildAvatarStackData(ctx context.Context, allParticipants []*git.CommitIdentity, emailUserMap *user.EmailUserMap) *AvatarStackData {
	if emailUserMap == nil {
		emails := make([]string, len(allParticipants))
		for i, sig := range allParticipants {
			emails[i] = sig.Email
		}
		var err error
		emailUserMap, err = user.GetUsersByEmails(ctx, emails)
		if err != nil {
			log.Error("GetUsersByEmails failed: %v", err)
		}
	}
	ret := &AvatarStackData{
		Participants: make([]*CommitParticipant, 0, len(allParticipants)),
	}
	uniqueUserIDs := make(container.Set[int64])
	for _, p := range allParticipants {
		var codenestUser *user.User
		if emailUserMap != nil {
			codenestUser = emailUserMap.GetByEmail(p.Email)
		}
		if codenestUser != nil {
			// identities without a CodeNest account can only be compared by their git identity
			if !uniqueUserIDs.Add(codenestUser.ID) {
				continue
			}
		}
		ret.Participants = append(ret.Participants, &CommitParticipant{CodeNestUser: codenestUser, GitIdentity: p})
	}
	return ret
}
