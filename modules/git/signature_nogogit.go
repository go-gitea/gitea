// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/util"
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

// Decode decodes a byte array representing a signature to signature
func (s *Signature) Decode(b []byte) {
	*s = *parseSignatureFromCommitLine(util.UnsafeBytesToString(b))
}
