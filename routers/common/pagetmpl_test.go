// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"
	"time"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	oauth2_source "gitea.dev/services/auth/source/oauth2"
	"gitea.dev/services/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuth2RequiredAdditionalInfoFailureWarning_AdminOnlyVisibility(t *testing.T) {
	c, err := cache.NewStringCache(setting.Cache{Adapter: "memory", Interval: 1})
	require.NoError(t, err)

	defer timeutil.MockSet(time.Unix(1_700_000_000, 0))()
	oauth2_source.SetRequiredAdditionalInfoFetchFailureWarning(c, "Google Workspace")

	adminCtx := &context.Context{
		Doer:  &user_model.User{IsAdmin: true},
		Cache: c,
	}
	nonAdminCtx := &context.Context{
		Doer:  &user_model.User{IsAdmin: false},
		Cache: c,
	}

	adminWarning := oauth2RequiredAdditionalInfoFailureWarning(adminCtx)
	require.NotNil(t, adminWarning)
	assert.Equal(t, "Google Workspace", adminWarning.SourceName)
	assert.Nil(t, oauth2RequiredAdditionalInfoFailureWarning(nonAdminCtx))
}
