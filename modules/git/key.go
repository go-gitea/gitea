// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

// Based on https://git-scm.com/docs/git-config#Documentation/git-config.txt-gpgformat
const (
	SigningKeyFormatOpenPGP = "openpgp" // for GPG keys, the expected default of git cli
	SigningKeyFormatSSH     = "ssh"
)

type SigningKey struct {
	KeyID  string
	Format string
}
