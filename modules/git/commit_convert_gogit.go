// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

func convertPGPSignature(c *object.Commit) *CommitGPGSignature {
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

	if c.Encoding != "" && c.Encoding != "UTF-8" {
		if _, err = fmt.Fprintf(&w, "\nencoding %s\n", c.Encoding); err != nil {
			return nil
		}
	}

	if _, err = fmt.Fprintf(&w, "\n\n%s", c.Message); err != nil {
		return nil
	}

	return &CommitGPGSignature{
		Signature: c.PGPSignature,
		Payload:   w.String(),
	}
}

func convertCommit(c *object.Commit) *Commit {
	return &Commit{
		ID:            ParseGogitHash(c.Hash),
		CommitMessage: c.Message,
		Committer:     &c.Committer,
		Author:        &c.Author,
		Signature:     convertPGPSignature(c),
		Parents:       ParseGogitHashArray(c.ParentHashes),
	}
}
