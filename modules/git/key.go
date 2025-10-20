// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
)

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

// GetSigningKey returns the KeyID and git Signature for the repo
func GetSigningKey(ctx context.Context, repoPath string) (*SigningKey, *Signature) {
	if setting.Repository.Signing.SigningKey == "none" {
		return nil, nil
	}

	if setting.Repository.Signing.SigningKey == "default" || setting.Repository.Signing.SigningKey == "" {
		// Can ignore the error here as it means that commit.gpgsign is not set
		value, _, _ := gitcmd.NewCommand("config", "--get", "commit.gpgsign").WithDir(repoPath).RunStdString(ctx)
		sign, valid := ParseBool(strings.TrimSpace(value))
		if !sign || !valid {
			return nil, nil
		}

		format, _, _ := gitcmd.NewCommand("config", "--default", SigningKeyFormatOpenPGP, "--get", "gpg.format").WithDir(repoPath).RunStdString(ctx)
		signingKey, _, _ := gitcmd.NewCommand("config", "--get", "user.signingkey").WithDir(repoPath).RunStdString(ctx)
		signingName, _, _ := gitcmd.NewCommand("config", "--get", "user.name").WithDir(repoPath).RunStdString(ctx)
		signingEmail, _, _ := gitcmd.NewCommand("config", "--get", "user.email").WithDir(repoPath).RunStdString(ctx)

		if strings.TrimSpace(signingKey) == "" {
			return nil, nil
		}

		return &SigningKey{
				KeyID:  strings.TrimSpace(signingKey),
				Format: strings.TrimSpace(format),
			}, &Signature{
				Name:  strings.TrimSpace(signingName),
				Email: strings.TrimSpace(signingEmail),
			}
	}

	if setting.Repository.Signing.SigningKey == "" {
		return nil, nil
	}

	return &SigningKey{
			KeyID:  setting.Repository.Signing.SigningKey,
			Format: setting.Repository.Signing.SigningFormat,
		}, &Signature{
			Name:  setting.Repository.Signing.SigningName,
			Email: setting.Repository.Signing.SigningEmail,
		}
}
