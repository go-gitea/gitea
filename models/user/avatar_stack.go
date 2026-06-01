// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/modules/git"
)

// AvatarStackUser is one participant in an avatar stack: a git identity, optionally
// matched to a Gitea user.
type AvatarStackUser struct {
	GiteaUser *User          // matched Gitea user, nil if unmatched
	Sig       *git.Signature // git identity (name/email)
}

// AvatarStackData is the view-model for the AvatarStack render helpers. Users[0] is the
// primary participant (commit author), painted on top; the rest follow.
type AvatarStackData struct {
	Users []*AvatarStackUser
}

// NewAvatarStackData builds a stack with the author first, followed by its co-authors.
func NewAvatarStackData(authorUser *User, authorSig *git.Signature, coAuthors []*AvatarStackUser) *AvatarStackData {
	if authorUser == nil && authorSig == nil {
		return nil
	}
	users := make([]*AvatarStackUser, 0, len(coAuthors)+1)
	users = append(users, &AvatarStackUser{GiteaUser: authorUser, Sig: authorSig})
	users = append(users, coAuthors...)
	return &AvatarStackData{Users: users}
}

// AvatarStackUsersFromSigs wraps each signature with the matching Gitea user if any.
func AvatarStackUsersFromSigs(sigs []*git.Signature, emailUserMap *EmailUserMap) []*AvatarStackUser {
	if len(sigs) == 0 {
		return nil
	}
	out := make([]*AvatarStackUser, len(sigs))
	for i, sig := range sigs {
		var giteaUser *User
		if emailUserMap != nil {
			giteaUser = emailUserMap.GetByEmail(sig.Email)
		}
		out[i] = &AvatarStackUser{GiteaUser: giteaUser, Sig: sig}
	}
	return out
}

// CoAuthorsFromCommit resolves a commit's co-author trailers into avatar-stack users.
func CoAuthorsFromCommit(ctx context.Context, c *git.Commit) ([]*AvatarStackUser, error) {
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
	return AvatarStackUsersFromSigs(sigs, emailUserMap), nil
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
