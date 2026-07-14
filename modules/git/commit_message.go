// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"net/mail"
	"regexp"
	"strings"
	"sync"

	"gitea.dev/modules/charset"
	"gitea.dev/modules/util"
)

// CoAuthoredByTrailer is the canonical token for the `Co-authored-by:` git trailer.
const CoAuthoredByTrailer = "Co-authored-by"

const (
	commitIdentityRoleAuthor    = 1
	commitIdentityRoleCommitter = 2
	commitIdentityRoleCoAuthor  = 3
)

type CommitIdentity struct {
	Name  string
	Email string
	role  int
}

// CommitMessageTrailerValues keys are all in lower-case
type CommitMessageTrailerValues map[string][]string

type CommitMessage struct {
	MessageRaw   string
	messageUTF8  *string
	messageTitle *string
	messageBody  *string

	trailerValues CommitMessageTrailerValues

	allParticipants      []*CommitIdentity
	committerCoAuthorIdx int
	committerCoAuthor    *CommitIdentity
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

	exclude := map[string]int{}
	addParticipant := func(name, email string, role int) (existingRole int) {
		if name == "" && email == "" {
			return 0
		}
		emailLower := strings.ToLower(email)
		if existingRole = exclude[emailLower]; emailLower != "" && existingRole != 0 {
			return existingRole
		}
		c.allParticipants = append(c.allParticipants, &CommitIdentity{Name: name, Email: email, role: role})
		exclude[emailLower] = role
		return 0
	}

	c.committerCoAuthorIdx = -1
	addParticipant(c.Author.Name, c.Author.Email, commitIdentityRoleAuthor)
	addParticipant(c.Committer.Name, c.Committer.Email, commitIdentityRoleCommitter)
	for _, coAuthorValue := range c.MessageTrailer()["co-authored-by"] {
		addr, err := mail.ParseAddress(coAuthorValue)
		coAuthorName, coAuthorEmail := coAuthorValue, ""
		if err == nil {
			coAuthorName, coAuthorEmail = addr.Name, addr.Address
		}
		existingRole := addParticipant(coAuthorName, coAuthorEmail, commitIdentityRoleCoAuthor)
		if existingRole == commitIdentityRoleCommitter && c.committerCoAuthorIdx == -1 {
			c.committerCoAuthorIdx = len(c.allParticipants)
			c.committerCoAuthor = &CommitIdentity{coAuthorName, coAuthorEmail, commitIdentityRoleCoAuthor}
		}
	}
	return c.allParticipants
}

// CoAuthorIdentities returns co-author identities defined by "Co-authored-by:" in the git message trailer
// Only the commit's author is excluded. If committer is declared as co-author, it will be included in the result.
// * Author & Co-author: they changed the code (attribution)
// * Committer: they submitted the commit but didn't change the code (e.g.: maintainer signed a commit)
// So, a committer can also be a co-author if they changed the code.
func (c *Commit) CoAuthorIdentities() (coAuthors []*CommitIdentity) {
	all := c.AllParticipantIdentities()
	if len(all) <= 1 {
		return nil // no co-author list
	}
	if all[1].role != commitIdentityRoleCommitter {
		return all[1:] // no committer, so all after author are co-authors
	}
	if c.committerCoAuthorIdx == -1 {
		return all[2:] // the committer is not in the co-author list, so just return the co-author list
	}
	// the committer is in the co-author list but de-duplicated, so include them as co-author again
	coAuthors = append(coAuthors, all[2:c.committerCoAuthorIdx]...)
	coAuthors = append(coAuthors, c.committerCoAuthor)
	coAuthors = append(coAuthors, all[c.committerCoAuthorIdx:]...)
	return coAuthors
}
