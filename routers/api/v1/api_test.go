// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1

import (
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoerNeedTwoFactorAuth(t *testing.T) {
	defer test.MockVariableValue(&setting.TwoFactorAuthEnforced, true)()

	for _, doer := range []*user_model.User{nil, user_model.NewActionsUser(), user_model.NewDeployKeyUser()} {
		need, err := doerNeedTwoFactorAuth(t.Context(), doer)
		require.NoError(t, err)
		assert.False(t, need)
	}
}
