// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

const (
	// KeyTypeOpenPGP is the key type for GPG keys
	KeyTypeOpenPGP = "openpgp"
	// KeyTypeSSH is the key type for SSH keys
	KeyTypeSSH = "ssh"
)

type SigningKey struct {
	KeyID  string
	Format string
}
