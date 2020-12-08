// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// GetDefaultPublicGPGKey returns the public key for a provided path
func GetDefaultPublicGPGKey(path string) (*service.GPGSettings, error) {
	gpgSettings := &service.GPGSettings{
		Sign: true,
	}

	value, _ := git.NewCommand("config", "--get", "commit.gpgsign").RunInDir(path)
	sign, valid := git.ParseBool(strings.TrimSpace(value))
	if !sign || !valid {
		gpgSettings.Sign = false
		return gpgSettings, nil
	}

	signingKey, _ := git.NewCommand("config", "--get", "user.signingkey").RunInDir(path)
	gpgSettings.KeyID = strings.TrimSpace(signingKey)

	defaultEmail, _ := git.NewCommand("config", "--get", "user.email").RunInDir(path)
	gpgSettings.Email = strings.TrimSpace(defaultEmail)

	defaultName, _ := git.NewCommand("config", "--get", "user.name").RunInDir(path)
	gpgSettings.Name = strings.TrimSpace(defaultName)

	if err := gpgSettings.LoadPublicKeyContent(); err != nil {
		return nil, err
	}

	return gpgSettings, nil
}
