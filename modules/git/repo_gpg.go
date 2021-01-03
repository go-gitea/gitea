// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
		return fmt.Errorf("Unable to get default signing key: %s, %s, %v", gpgSettings.KeyID, stderr, err)
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

	value, _ := NewCommand("config", "--get", "commit.gpgsign").RunInDir(repo.Path)
	sign, valid := ParseBool(strings.TrimSpace(value))
	if !sign || !valid {
		gpgSettings.Sign = false
		repo.gpgSettings = gpgSettings
		return gpgSettings, nil
	}

	signingKey, _ := NewCommand("config", "--get", "user.signingkey").RunInDir(repo.Path)
	gpgSettings.KeyID = strings.TrimSpace(signingKey)

	defaultEmail, _ := NewCommand("config", "--get", "user.email").RunInDir(repo.Path)
	gpgSettings.Email = strings.TrimSpace(defaultEmail)

	defaultName, _ := NewCommand("config", "--get", "user.name").RunInDir(repo.Path)
	gpgSettings.Name = strings.TrimSpace(defaultName)

	if err := gpgSettings.LoadPublicKeyContent(); err != nil {
		return nil, err
	}
	repo.gpgSettings = gpgSettings
	return repo.gpgSettings, nil
}
