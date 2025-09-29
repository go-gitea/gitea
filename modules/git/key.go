// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "code.gitea.io/gitea/modules/setting"

// Based on https://git-scm.com/docs/git-config#Documentation/git-config.txt-gpgformat
const (
	SigningKeyFormatOpenPGP = "openpgp" // for GPG keys, the expected default of git cli
	SigningKeyFormatSSH     = "ssh"
)

// SigningKey represents an instance key info which will be used to sign git commits.
// FIXME: need to refactor it to a new name, this name conflicts with the variable names for "asymkey.GPGKey" in many places.
type SigningKey struct {
	KeyID  string
	Format string
}

func (s *SigningKey) String() string {
	// Do not expose KeyID
	// In case the key is a file path and the struct is rendered in a template, then the server path will be exposed.
	setting.PanicInDevOrTesting("don't call SigningKey.String() - it exposes the KeyID which might be a local file path")
	return "SigningKey:" + s.Format
}
