// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gituser

import (
	"context"

	"gitea.dev/models/user"
	"gitea.dev/modules/git"
)

// AvatarStackData is the view-model for the AvatarStack render helpers. Participants[0] is
// the primary participant (commit author), painted on top; the rest follow.
type AvatarStackData struct {
	Participants []*CommitParticipant
}

// NewAvatarStackData builds a stack with the author first, followed by its co-authors.
func NewAvatarStackData(authorUser *user.User, authorSig *git.Signature, coAuthors []*CommitParticipant) *AvatarStackData {
	if authorUser == nil && authorSig == nil {
		return nil
	}
	participants := make([]*CommitParticipant, 0, len(coAuthors)+1)
	participants = append(participants, &CommitParticipant{GiteaUser: authorUser, GitIdentity: authorSig})
	participants = append(participants, coAuthors...)
	return &AvatarStackData{Participants: participants}
}

// CommitParticipantsFromSigs wraps each signature with the matching Gitea user if any.
func CommitParticipantsFromSigs(sigs []*git.Signature, emailUserMap *user.EmailUserMap) []*CommitParticipant {
	if len(sigs) == 0 {
		return nil
	}
	out := make([]*CommitParticipant, len(sigs))
	for i, sig := range sigs {
		var giteaUser *user.User
		if emailUserMap != nil {
			giteaUser = emailUserMap.GetByEmail(sig.Email)
		}
		out[i] = &CommitParticipant{GiteaUser: giteaUser, GitIdentity: sig}
	}
	return out
}

// GetAllCommitParticipants resolves a commit's co-author trailers into avatar-stack users.
func GetAllCommitParticipants(ctx context.Context, c *git.Commit) ([]*CommitParticipant, error) {
	sigs := c.AllAuthorSignatures()
	if len(sigs) == 0 {
		return nil, nil
	}
	emails := make([]string, len(sigs))
	for i, sig := range sigs {
		emails[i] = sig.Email
	}
	emailUserMap, err := user.GetUsersByEmails(ctx, emails)
	if err != nil {
		return nil, err
	}
	return CommitParticipantsFromSigs(sigs, emailUserMap), nil
}
