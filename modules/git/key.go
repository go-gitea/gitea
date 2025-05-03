// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

// Based on https://git-scm.com/docs/git-config#Documentation/git-config.txt-gpgformat
const (
	// KeyTypeOpenPGP is the key type for GPG keys, expected default of git cli
	KeyTypeOpenPGP = "openpgp"
	// KeyTypeSSH is the key type for SSH keys
	KeyTypeSSH = "ssh"
)

type SigningKey struct {
	KeyID  string
	Format string
}
