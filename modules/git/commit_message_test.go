// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitMessageSanitizesInvalidUTF8(t *testing.T) {
	commit := &Commit{
		CommitMessage: CommitMessage{MessageRaw: "title \xff\n\n\n\nbody \xff\n\n\n"},
	}
	assert.Equal(t, "title ÿ", commit.MessageTitle())
	assert.Equal(t, "body ÿ", commit.MessageBody())
	assert.Equal(t, "title ÿ\n\n\n\nbody ÿ\n\n\n", commit.MessageUTF8())
}

func TestCommitMessageTrailer(t *testing.T) {
	cases := []struct {
		msg, body, sep, trailer string
	}{
		{"", "", "", ""},
		{"a", "a", "", ""},
		{"a\n\nk", "a\n\nk", "", ""},
		{"a\n\nk:v", "a", "\n\n", "k:v"},
		{"a\n\nk:v\n\n", "a", "\n\n", "k:v\n\n"},
		{"a\n--\nk:v", "a\n--\nk:v", "", ""},
		{"a\n---\nk:v", "a", "\n---\n", "k:v"},
		{"a\n\n---\n\nk:v", "a\n", "\n---\n\n", "k:v"},

		{"k: v", "", "", "k: v"},
		{"\nk:v", "", "\n", "k:v"},
		{"\n\nk:v", "", "\n\n", "k:v"},

		{"---\nk:v", "", "---\n", "k:v"},
		{"\n---\nk:v", "", "\n---\n", "k:v"},
		{"a:b\n---\nk:v", "a:b", "\n---\n", "k:v"},
	}
	for _, c := range cases {
		body, sep, trailer := CommitMessageSplitTrailer(c.msg)
		assert.Equal(t, c.body, body, "input=%q", c.msg)
		assert.Equal(t, c.sep, sep, "input=%q", c.msg)
		assert.Equal(t, c.trailer, trailer, "input=%q", c.msg)
	}
}

func TestCommitMessageAllParticipantIdentities(t *testing.T) {
	sig := func(n, e string) *Signature { return &Signature{Name: n, Email: e} }
	idt := func(n, e string, r int) *CommitIdentity { return &CommitIdentity{n, e, r} }
	cases := []struct {
		commit      *Commit
		participant []*CommitIdentity
	}{
		{
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
				CommitMessage: CommitMessage{MessageRaw: "CO-Authored-BY: x@m.com"},
			},
			[]*CommitIdentity{idt("a", "a@m.com", commitIdentityRoleAuthor), idt("c", "c@m.com", commitIdentityRoleCommitter), idt("", "x@m.com", commitIdentityRoleCoAuthor)},
		},
		{
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("a", "A@M.com"),
				CommitMessage: CommitMessage{MessageRaw: "CO-Authored-BY: a@m.com"},
			},
			[]*CommitIdentity{idt("a", "a@m.com", commitIdentityRoleAuthor)},
		},
		{
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("", ""),
				CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: Full Name <X@M.com>"},
			},
			[]*CommitIdentity{idt("a", "a@m.com", commitIdentityRoleAuthor), idt("Full Name", "X@M.com", commitIdentityRoleCoAuthor)},
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.participant, c.commit.AllParticipantIdentities())
	}
}

func TestCommitMessageCoAuthorIdentities(t *testing.T) {
	sig := func(n, e string) *Signature { return &Signature{Name: n, Email: e} }
	idt := func(n, e string, r int) *CommitIdentity { return &CommitIdentity{n, e, r} }
	cases := []struct {
		name      string
		commit    *Commit
		coAuthors []*CommitIdentity
	}{
		{
			"GenuineCoAuthor",
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
				CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: x <x@m.com>"},
			},
			[]*CommitIdentity{idt("x", "x@m.com", commitIdentityRoleCoAuthor)},
		},
		{
			"CoAuthorIsCommitter",
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
				CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: c <c@m.com>"},
			},
			[]*CommitIdentity{idt("c", "c@m.com", commitIdentityRoleCoAuthor)},
		},
		{
			"CoAuthorIsAuthor",
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
				CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: a <a@m.com>"},
			},
			[]*CommitIdentity{},
		},
		{
			"CoAuthorCommitterNameWithIndex", // restore the committer co-author to the co-author list by the index with correct name
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
				CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: x <x@m.com>\nCo-authored-by: c-other <c@m.com>\nCo-authored-by: y <y@m.com>"},
			},
			[]*CommitIdentity{
				idt("x", "x@m.com", commitIdentityRoleCoAuthor),
				idt("c-other", "c@m.com", commitIdentityRoleCoAuthor),
				idt("y", "y@m.com", commitIdentityRoleCoAuthor),
			},
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.coAuthors, c.commit.CoAuthorIdentities(), "case: %s", c.name)
	}
}
