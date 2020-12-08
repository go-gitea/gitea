// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git/providers/native"
	"code.gitea.io/gitea/modules/git/service"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func convertPGPSignature(c *object.Commit) *service.GPGSignature {
	if c.PGPSignature == "" {
		return nil
	}

	var w strings.Builder
	var err error

	if _, err = fmt.Fprintf(&w, "tree %s\n", c.TreeHash.String()); err != nil {
		return nil
	}

	for _, parent := range c.ParentHashes {
		if _, err = fmt.Fprintf(&w, "parent %s\n", parent.String()); err != nil {
			return nil
		}
	}

	if _, err = fmt.Fprint(&w, "author "); err != nil {
		return nil
	}

	if err = c.Author.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprint(&w, "\ncommitter "); err != nil {
		return nil
	}

	if err = c.Committer.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "\n\n%s", c.Message); err != nil {
		return nil
	}

	return &service.GPGSignature{
		Signature: c.PGPSignature,
		Payload:   w.String(),
	}
}

func convertCommit(repo service.Repository, c *object.Commit) service.Commit {
	return native.NewCommit(
		&Object{
			hash: fromPlumbingHash(c.Hash),
			repo: repo,
		},
		fromPlumbingHash(c.TreeHash),
		nil,
		convertSignature(&c.Committer),
		convertSignature(&c.Author),
		convertPGPSignature(c),
		FromPlumbingHashes(c.ParentHashes),
		c.Message,
	)
}
