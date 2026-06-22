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
	idt := func(n, e string) *CommitIdentity { return &CommitIdentity{Name: n, Email: e} }
	cases := []struct {
		commit      *Commit
		participant []*CommitIdentity
	}{
		{
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
				CommitMessage: CommitMessage{MessageRaw: "CO-Authored-BY: x@m.com"},
			},
			[]*CommitIdentity{idt("a", "a@m.com"), idt("c", "c@m.com"), idt("", "x@m.com")},
		},
		{
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("a", "A@M.com"),
				CommitMessage: CommitMessage{MessageRaw: "CO-Authored-BY: a@m.com"},
			},
			[]*CommitIdentity{idt("a", "a@m.com")},
		},
		{
			&Commit{
				Author: sig("a", "a@m.com"), Committer: sig("", ""),
				CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: Full Name <X@M.com>"},
			},
			[]*CommitIdentity{idt("a", "a@m.com"), idt("Full Name", "X@M.com")},
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.participant, c.commit.AllParticipantIdentities())
	}
}
