// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
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

	stdout := new(bytes.Buffer)
	_ = NewCommand(repo.Ctx, "config", "--get", "commit.gpgsign").RunWithContext(&RunContext{Dir: repo.Path, Timeout: -1, Stdout: stdout})
	sign, valid := ParseBool(strings.TrimSpace(stdout.String()))
	if !sign || !valid {
		gpgSettings.Sign = false
		repo.gpgSettings = gpgSettings
		return gpgSettings, nil
	}

	stdout.Reset()
	_ = NewCommand(repo.Ctx, "config", "--get", "user.signingkey").RunWithContext(&RunContext{Dir: repo.Path, Timeout: -1, Stdout: stdout})
	gpgSettings.KeyID = strings.TrimSpace(stdout.String())

	stdout.Reset()
	_ = NewCommand(repo.Ctx, "config", "--get", "user.email").RunWithContext(&RunContext{Dir: repo.Path, Timeout: -1, Stdout: stdout})
	gpgSettings.Email = strings.TrimSpace(stdout.String())

	stdout.Reset()
	_ = NewCommand(repo.Ctx, "config", "--get", "user.name").RunWithContext(&RunContext{Dir: repo.Path, Timeout: -1, Stdout: stdout})
	gpgSettings.Name = strings.TrimSpace(stdout.String())

	if err := gpgSettings.LoadPublicKeyContent(); err != nil {
		return nil, err
	}
	repo.gpgSettings = gpgSettings
	return repo.gpgSettings, nil
}
