// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/modules/git"
)

// CommitParticipant is one participant of a commit (its author or a co-author): a git
// identity, optionally matched to a Gitea user.
type CommitParticipant struct {
	GiteaUser *User          // matched Gitea user, nil if unmatched
	Sig       *git.Signature // git identity (name/email)
}

// AvatarStackData is the view-model for the AvatarStack render helpers. Participants[0] is
// the primary participant (commit author), painted on top; the rest follow.
type AvatarStackData struct {
	Participants []*CommitParticipant
}

// NewAvatarStackData builds a stack with the author first, followed by its co-authors.
func NewAvatarStackData(authorUser *User, authorSig *git.Signature, coAuthors []*CommitParticipant) *AvatarStackData {
	if authorUser == nil && authorSig == nil {
		return nil
	}
	participants := make([]*CommitParticipant, 0, len(coAuthors)+1)
	participants = append(participants, &CommitParticipant{GiteaUser: authorUser, Sig: authorSig})
	participants = append(participants, coAuthors...)
	return &AvatarStackData{Participants: participants}
}

// CommitParticipantsFromSigs wraps each signature with the matching Gitea user if any.
func CommitParticipantsFromSigs(sigs []*git.Signature, emailUserMap *EmailUserMap) []*CommitParticipant {
	if len(sigs) == 0 {
		return nil
	}
	out := make([]*CommitParticipant, len(sigs))
	for i, sig := range sigs {
		var giteaUser *User
		if emailUserMap != nil {
			giteaUser = emailUserMap.GetByEmail(sig.Email)
		}
		out[i] = &CommitParticipant{GiteaUser: giteaUser, Sig: sig}
	}
	return out
}

// CoAuthorsFromCommit resolves a commit's co-author trailers into avatar-stack users.
func CoAuthorsFromCommit(ctx context.Context, c *git.Commit) ([]*CommitParticipant, error) {
	sigs := c.CoAuthorSignatures()
	if len(sigs) == 0 {
		return nil, nil
	}
	emails := make([]string, len(sigs))
	for i, sig := range sigs {
		emails[i] = sig.Email
	}
	emailUserMap, err := GetUsersByEmails(ctx, emails)
	if err != nil {
		return nil, err
	}
	return CommitParticipantsFromSigs(sigs, emailUserMap), nil
}

// AvatarStackData returns the view-model for rendering this commit's author + co-authors.
func (uc *UserCommit) AvatarStackData() *AvatarStackData {
	if uc == nil {
		return nil
	}
	var sig *git.Signature
	if uc.Commit != nil {
		sig = uc.Commit.Author
	}
	return NewAvatarStackData(uc.User, sig, uc.CoAuthors)
}
