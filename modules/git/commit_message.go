// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"net/mail"
	"regexp"
	"strings"
	"sync"

	"gitea.dev/modules/charset"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"
)

// CoAuthoredByTrailer is the canonical token for the `Co-authored-by:` git trailer.
const CoAuthoredByTrailer = "Co-authored-by"

type CommitIdentity struct {
	Name  string
	Email string
}

// CommitMessageTrailerValues keys are all in lower-case
type CommitMessageTrailerValues map[string][]string

type CommitMessage struct {
	MessageRaw   string
	messageUTF8  *string
	messageTitle *string
	messageBody  *string

	trailerValues CommitMessageTrailerValues

	allParticipants []*CommitIdentity
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

func (c *CommitMessage) MessageTrailer() CommitMessageTrailerValues {
	if c.trailerValues == nil {
		_, _, trailer := CommitMessageSplitTrailer(c.MessageUTF8())
		c.trailerValues = CommitMessageParseTrailer(trailer)
	}
	return c.trailerValues
}

var commitMessageTrailerSplit = sync.OnceValue(func() *regexp.Regexp {
	// the sep is either something like "\n---\n" or "\n\n" in the body, or at the start of the body like "---\n"
	return regexp.MustCompile(`(?s)^(?P<content>.*?)(?P<sep>^|^\n|^-{3,}\n+|\n-{3,}\n+|\n\n)(?P<trailer>(?:[A-Za-z0-9][-A-Za-z0-9]*:[^\n]*\n?)*\n*)$`)
})

// CommitMessageSplitTrailer tries to split the message by the trailer separator
// content + sep + trailer will reconstruct the original message
func CommitMessageSplitTrailer(s string) (content, sep, trailer string) {
	s = util.NormalizeStringEOL(s)
	re := commitMessageTrailerSplit()
	v := re.FindStringSubmatch(s)
	if v == nil {
		return s, "", ""
	}
	return v[re.SubexpIndex("content")], v[re.SubexpIndex("sep")], v[re.SubexpIndex("trailer")]
}

func CommitMessageParseTrailer(s string) CommitMessageTrailerValues {
	ret := CommitMessageTrailerValues{}
	for line := range strings.SplitSeq(util.NormalizeStringEOL(s), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		kLower := strings.ToLower(k)
		ret[kLower] = append(ret[kLower], v)
	}
	return ret
}

// AllParticipantIdentities returns all the participants in the commit, the first one is the commit's author
func (c *Commit) AllParticipantIdentities() []*CommitIdentity {
	if c.allParticipants != nil {
		return c.allParticipants
	}

	exclude := container.Set[string]{}
	c.allParticipants = append(c.allParticipants, &CommitIdentity{Name: c.Author.Name, Email: c.Author.Email})
	exclude.Add(strings.ToLower(c.Author.Email))

	addParticipant := func(name, email string) {
		if name == "" && email == "" {
			return
		}
		emailLower := strings.ToLower(email)
		if emailLower != "" && exclude.Contains(emailLower) {
			return
		}
		c.allParticipants = append(c.allParticipants, &CommitIdentity{Name: name, Email: email})
		exclude.Add(emailLower)
	}
	addParticipant(c.Committer.Name, c.Committer.Email)
	for _, coAuthorValue := range c.MessageTrailer()["co-authored-by"] {
		addr, err := mail.ParseAddress(coAuthorValue)
		if err == nil {
			addParticipant(addr.Name, addr.Address)
		} else {
			addParticipant(coAuthorValue, "")
		}
	}
	return c.allParticipants
}
