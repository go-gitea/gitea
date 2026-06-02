// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"net/mail"
	"strings"

	"gitea.dev/modules/charset"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"
)

// CoAuthoredByTrailer is the canonical token for the `Co-authored-by:` git trailer.
const CoAuthoredByTrailer = "Co-authored-by"

type CommitIdentity struct {
	Name  string
	Email string

	ParticipantRole string
}

type CommitMessage struct {
	MessageRaw   string
	messageUTF8  *string
	messageTitle *string
	messageBody  *string

	messageBodyContent *string
	messageBodySep     *string
	messageBodyTrailer *string

	coAuthors []*CommitIdentity
}

func (c *CommitMessage) MessageUTF8() string {
	if c.messageUTF8 == nil {
		bs := charset.ToUTF8(util.UnsafeStringToBytes(c.MessageRaw), charset.ConvertOpts{ErrorReplacement: []byte{'?'}})
		c.messageUTF8 = new(util.UnsafeBytesToString(bs))
	}
	return *c.messageUTF8
}

func (c *CommitMessage) MessageTitle() string {
	if c.messageTitle == nil {
		s, _, _ := strings.Cut(strings.TrimSpace(c.MessageUTF8()), "\n")
		c.messageTitle = new(strings.TrimSpace(s))
	}
	return *c.messageTitle
}

func (c *CommitMessage) MessageBody() string {
	if c.messageBody == nil {
		_, s, _ := strings.Cut(strings.TrimSpace(c.MessageUTF8()), "\n")
		c.messageBody = new(strings.TrimSpace(s))
	}
	return *c.messageBody
}

// isTrailerLineShape reports whether the line matches `[A-Za-z0-9-]+:<non-empty>`
// per `git interpret-trailers`.
func isTrailerLineShape(line string) bool {
	token, rest, ok := strings.Cut(line, ":")
	if !ok || token == "" || strings.TrimSpace(rest) == "" {
		return false
	}
	for _, r := range token {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

// parseCoAuthorTrailer extracts a co-author from a `Co-authored-by:` trailer
// (case-insensitive). Returns false on a malformed address.
func parseCoAuthorTrailer(line string) (*CommitIdentity, bool) {
	token, rest, ok := strings.Cut(line, ":")
	if !ok {
		return nil, false
	}
	if !strings.EqualFold(token, CoAuthoredByTrailer) {
		return nil, false
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(rest))
	if err != nil {
		return nil, false
	}
	name := addr.Name
	if name == "" {
		name = addr.Address
	}
	return &CommitIdentity{Name: name, Email: addr.Address}, true
}

// parseCoAuthorSignatures parses `Co-authored-by:` trailers from the trailing
// block of the commit message. Only the last paragraph is scanned (and it must
// contain only trailer-shaped lines) so in-body occurrences inside a revert or
// cherry-pick description are not misinterpreted as trailers.
func (c *CommitMessage) parseCoAuthorSignatures() []*CommitIdentity {
	if c.coAuthors != nil {
		return c.coAuthors
	}
	body := strings.TrimRight(util.NormalizeStringEOL(c.MessageBody()), "\n")
	if idx := strings.LastIndex(body, "\n\n"); idx >= 0 {
		body = body[idx+2:]
	}
	c.coAuthors = []*CommitIdentity{}
	seen := container.Set[string]{}
	for line := range strings.SplitSeq(body, "\n") {
		if !isTrailerLineShape(line) {
			break
		}
		sig, ok := parseCoAuthorTrailer(line)
		if !ok {
			continue
		}
		if !seen.Add(strings.ToLower(sig.Email)) {
			continue
		}
		c.coAuthors = append(c.coAuthors, sig)
	}
	return c.coAuthors
}

// AllParticipantIdentities returns all the participants in the commit
func (c *Commit) AllParticipantIdentities() (out []*CommitIdentity) {
	exclude := container.Set[string]{}
	if c.Author.Name != "" || c.Author.Email != "" {
		out = append(out, &CommitIdentity{Name: c.Author.Name, Email: c.Author.Email, ParticipantRole: "author"})
		exclude.Add(strings.ToLower(c.Author.Email))
	}
	if c.Committer.Name != "" || c.Committer.Email != "" {
		if !exclude.Contains(strings.ToLower(c.Committer.Email)) {
			out = append(out, &CommitIdentity{Name: c.Committer.Name, Email: c.Committer.Email, ParticipantRole: "committer"})
			exclude.Add(strings.ToLower(c.Committer.Email))
		}
	}
	for _, ca := range c.parseCoAuthorSignatures() {
		if !exclude.Contains(strings.ToLower(ca.Email)) {
			out = append(out, &CommitIdentity{Name: ca.Name, Email: ca.Email, ParticipantRole: "co-author"})
		}
	}
	return out
}
