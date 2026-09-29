// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"fmt"
	"io"
	"time"

	"gitea.dev/modules/util"
)

// Signature represents the Author, Committer or Tagger information.
type Signature struct {
	Name  string    // the committer name, it can be anything
	Email string    // the committer email, it can be anything
	When  time.Time // the timestamp of the signature
}

func (s *Signature) String() string {
	return fmt.Sprintf("%s <%s>", s.Name, s.Email)
}

// Encode writes the signature for git commit object (same as gogit's object.Signature Encode method)
func (s *Signature) Encode(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s <%s> %d %s", s.Name, s.Email, max(0, s.When.Unix()), s.When.Format("-0700"))
	return err
}

// Decode parses the signature for git commit object (same as gogit's object.Signature Decode method)
func (s *Signature) Decode(b []byte) {
	*s = *parseSignatureFromCommitLine(util.UnsafeBytesToString(b))
}
