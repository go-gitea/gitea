// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"context"
	"errors"

	codespace_model "gitea.dev/models/codespace"
)

func requireManager(ctx context.Context) (*codespace_model.Manager, error) {
	manager := GetManager(ctx)
	if manager == nil {
		return nil, errors.New("manager not authenticated")
	}
	return manager, nil
}
