// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_provider

import (
	"context"

	"code.gitea.io/gitea/modules/setting"
)

// Init initializes the oauth source
func Init(ctx context.Context) error {
	if !setting.OAuth2.Enabled {
		return nil
	}

	if err := InitSigningKey(); err != nil {
		return err
	}

	return nil
}
