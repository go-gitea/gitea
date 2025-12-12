// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/process"
)

// GPGSettings represents the default GPG settings for this repository
type GPGSettings struct {
	Sign             bool
	KeyID            string
	Email            string
	Name             string
	PublicKeyContent string
	Format           string
}

// LoadPublicKeyContent will load the key from gpg
func (gpgSettings *GPGSettings) LoadPublicKeyContent() error {
	if gpgSettings.Format == SigningKeyFormatSSH {
		content, err := os.ReadFile(gpgSettings.KeyID)
		if err != nil {
			return fmt.Errorf("unable to read SSH public key file: %s, %w", gpgSettings.KeyID, err)
		}
		gpgSettings.PublicKeyContent = string(content)
		return nil
	}
	content, stderr, err := process.GetManager().Exec(
		"gpg -a --export",
		"gpg", "-a", "--export", gpgSettings.KeyID)
	if err != nil {
		return fmt.Errorf("unable to get default signing key: %s, %s, %w", gpgSettings.KeyID, stderr, err)
	}
	gpgSettings.PublicKeyContent = content
	return nil
}

// GetDefaultPublicGPGKey will return and cache the default public GPG settings for this repository
func GetDefaultPublicGPGKey(ctx context.Context, forceUpdate bool) (*GPGSettings, error) {
	gpgSettings := &GPGSettings{
		Sign: true,
	}

	value, _, _ := gitcmd.NewCommand("config", "--global", "--get", "commit.gpgsign").RunStdString(ctx)
	sign, valid := ParseBool(strings.TrimSpace(value))
	if !sign || !valid {
		gpgSettings.Sign = false
		return gpgSettings, nil
	}

	signingKey, _, _ := gitcmd.NewCommand("config", "--global", "--get", "user.signingkey").RunStdString(ctx)
	gpgSettings.KeyID = strings.TrimSpace(signingKey)

	format, _, _ := gitcmd.NewCommand("config", "--global", "--default", SigningKeyFormatOpenPGP, "--get", "gpg.format").RunStdString(ctx)
	gpgSettings.Format = strings.TrimSpace(format)

	defaultEmail, _, _ := gitcmd.NewCommand("config", "--global", "--get", "user.email").RunStdString(ctx)
	gpgSettings.Email = strings.TrimSpace(defaultEmail)

	defaultName, _, _ := gitcmd.NewCommand("config", "--global", "--get", "user.name").RunStdString(ctx)
	gpgSettings.Name = strings.TrimSpace(defaultName)

	if err := gpgSettings.LoadPublicKeyContent(); err != nil {
		return nil, err
	}
	return gpgSettings, nil
}
