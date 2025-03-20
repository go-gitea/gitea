// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/process"
)

// LoadPublicKeyContent will load the key from gpg
func (gpgSettings *GPGSettings) LoadPublicKeyContent() error {
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
func (repo *Repository) GetDefaultPublicGPGKey(forceUpdate bool) (*GPGSettings, error) {
	if repo.gpgSettings != nil && !forceUpdate {
		return repo.gpgSettings, nil
	}

	gpgSettings := &GPGSettings{
		Sign: true,
	}

	value, _, _ := NewCommand("config", "--get", "commit.gpgsign").RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	sign, valid := ParseBool(strings.TrimSpace(value))
	if !sign || !valid {
		gpgSettings.Sign = false
		repo.gpgSettings = gpgSettings
		return gpgSettings, nil
	}

	signingKey, _, _ := NewCommand("config", "--get", "user.signingkey").RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	gpgSettings.KeyID = strings.TrimSpace(signingKey)

	defaultEmail, _, _ := NewCommand("config", "--get", "user.email").RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	gpgSettings.Email = strings.TrimSpace(defaultEmail)

	defaultName, _, _ := NewCommand("config", "--get", "user.name").RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	gpgSettings.Name = strings.TrimSpace(defaultName)

	if err := gpgSettings.LoadPublicKeyContent(); err != nil {
		return nil, err
	}
	repo.gpgSettings = gpgSettings
	return repo.gpgSettings, nil
}
