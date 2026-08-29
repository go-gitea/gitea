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
		{"a\n\nk:v\n  next-line", "a", "\n\n", "k:v\n  next-line"},
		{"a\n\nk:v\n  next-line\nother: v", "a", "\n\n", "k:v\n  next-line\nother: v"},
		{"a\n\nk:v\n\n", "a", "\n\n", "k:v\n\n"},
		{"a\n--\nk:v", "a\n--\nk:v", "", ""},
		{"a\n---\nk:v", "a", "\n---\n", "k:v"}, // TODO: should we support such case? No empty line between "---" and the trailer
		{"a\n\n---\n\nk:v", "a", "\n\n---\n\n", "k:v"},

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

func TestCommitMessageParticipants(t *testing.T) {
	sig := func(n, e string) *Signature { return &Signature{Name: n, Email: e} }
	idt := func(n, e string, r int) *CommitIdentity { return &CommitIdentity{n, e, r} }
	roleAuthor, _, roleCoAuthor := commitIdentityRoleAuthor, commitIdentityRoleCommitter, commitIdentityRoleCoAuthor
	type testCase struct {
		name       string
		commit     *Commit
		identities []*CommitIdentity
	}

	t.Run("AllAuthors", func(t *testing.T) {
		cases := []testCase{
			{
				"CommitterExcluded",
				&Commit{
					Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
					CommitMessage: CommitMessage{MessageRaw: "CO-Authored-BY: Full Name <x@m.com>"},
				},
				[]*CommitIdentity{idt("a", "a@m.com", roleAuthor), idt("Full Name", "x@m.com", roleCoAuthor)},
			},
			{
				"AuthorIsCoAuthor",
				&Commit{
					Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
					CommitMessage: CommitMessage{MessageRaw: "CO-Authored-BY: other-name <a@m.com>"},
				},
				[]*CommitIdentity{idt("a", "a@m.com", roleAuthor)},
			},
			{
				"EmptyAuthor", // synthesized commits (push feed) may have no author signature at all
				&Commit{
					Author: sig("", ""), Committer: sig("", ""),
					CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: c <c@m.com>"},
				},
				// but if the commit message contains co-authors, the co-authors are still parsed for "all authors"
				// if it is a problem, the caller should fix the problem (provide correct "author")
				[]*CommitIdentity{idt("c", "c@m.com", roleCoAuthor)},
			},
		}
		for _, c := range cases {
			assert.Equal(t, c.identities, c.commit.AllAuthorIdentities(), "case: %s", c.name)
		}
	})
	t.Run("CoAuthors", func(t *testing.T) {
		cases := []testCase{
			{
				"GenuineCoAuthor",
				&Commit{
					Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
					CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: x <x@m.com>"},
				},
				[]*CommitIdentity{idt("x", "x@m.com", roleCoAuthor)},
			},
			{
				"CoAuthorIsCommitter",
				&Commit{
					Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
					CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: c <c@m.com>"},
				},
				[]*CommitIdentity{idt("c", "c@m.com", roleCoAuthor)},
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
				"CoAuthorNameOnlyAndDuplicate",
				&Commit{
					Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
					CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: b\nCo-authored-by: b\nCo-authored-by: c"},
				},
				[]*CommitIdentity{idt("b", "", roleCoAuthor), idt("c", "", roleCoAuthor)},
			},
			{
				"CoAuthorNameNotAnEmailAddress", // names net/mail rejects, e.g. bots and names with a comma
				&Commit{
					Author: sig("a", "a@m.com"), Committer: sig("c", "c@m.com"),
					CommitMessage: CommitMessage{MessageRaw: "Co-authored-by: dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>\nCo-authored-by: Smith, John <j@m.com>"},
				},
				[]*CommitIdentity{
					idt("dependabot[bot]", "49699333+dependabot[bot]@users.noreply.github.com", roleCoAuthor),
					idt("Smith, John", "j@m.com", roleCoAuthor),
				},
			},
		}
		for _, c := range cases {
			assert.Equal(t, c.identities, c.commit.CoAuthorIdentities(), "case: %s", c.name)
		}
	})
}

func TestCommitMessageMerge(t *testing.T) {
	cases := []struct {
		m1, m2 string
		out    string
	}{
		{"", "", ""},
		{"msg1", "", "msg1"},
		{"", "msg2", "msg2"},
		{"msg1", "msg2", "msg1\n\nmsg2"},
		{"k1: a", "", "k1: a"},
		{"", "k2: b", "k2: b"},
		{"k1: a", "k2: b", "k1: a\nk2: b"},
		{"msg1", "k2: b", "msg1\n\nk2: b"},
		{"k1: a", "msg2", "msg2\n\nk1: a"},
		{"msg1\n\nk1: a", "msg2", "msg1\n\nmsg2\n\nk1: a"},
		{"msg1\n----\nk1: a", "msg2", "msg1\n\nmsg2\n----\nk1: a"},
		{"msg1\n\n----\n\nk1: a", "msg2", "msg1\n\nmsg2\n\n----\n\nk1: a"},
		{"msg1", "msg2\n----\nk2: b", "msg1\n\nmsg2\n----\nk2: b"},
		{"msg1", "msg2\n\nk2: b", "msg1\n\nmsg2\n\nk2: b"},
		{"msg1\n\nk1: a", "msg2\n\nk2: b", "msg1\n\nmsg2\n\nk1: a\nk2: b"},
	}

	for i, c := range cases {
		out := CommitMessageMerge(c.m1, c.m2)
		assert.Equal(t, c.out, out, "idx=%d, m1=%q m2=%q", i, c.m1, c.m2)
	}
}
