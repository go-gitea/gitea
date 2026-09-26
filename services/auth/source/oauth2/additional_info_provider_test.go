// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"testing"
	"time"

	"gitea.dev/modules/cache"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsGoogleGroupClaimRequiredForLoginFlow(t *testing.T) {
	t.Run("no group-dependent options", func(t *testing.T) {
		source := &Source{
			GroupClaimName: "groups",
		}
		assert.False(t, isGoogleGroupClaimRequiredForLoginFlow(source))
	})

	t.Run("required claim uses group claim", func(t *testing.T) {
		source := &Source{
			GroupClaimName:    "custom_groups",
			RequiredClaimName: "custom_groups",
		}
		assert.True(t, isGoogleGroupClaimRequiredForLoginFlow(source))
	})

	t.Run("required claim uses default groups claim", func(t *testing.T) {
		source := &Source{
			RequiredClaimName: "groups",
		}
		assert.True(t, isGoogleGroupClaimRequiredForLoginFlow(source))
	})

	t.Run("admin group configured", func(t *testing.T) {
		source := &Source{
			AdminGroup: "admins@example.com",
		}
		assert.False(t, isGoogleGroupClaimRequiredForLoginFlow(source))
	})

	t.Run("restricted group configured", func(t *testing.T) {
		source := &Source{
			RestrictedGroup: "restricted@example.com",
		}
		assert.False(t, isGoogleGroupClaimRequiredForLoginFlow(source))
	})

	t.Run("group team mapping configured", func(t *testing.T) {
		source := &Source{
			GroupTeamMap: "{\"a\": {\"org\": [\"team\"]}}",
		}
		assert.False(t, isGoogleGroupClaimRequiredForLoginFlow(source))
	})

	t.Run("group team mapping removal enabled", func(t *testing.T) {
		source := &Source{
			GroupTeamMapRemoval: true,
		}
		assert.False(t, isGoogleGroupClaimRequiredForLoginFlow(source))
	})
}

func TestRequiredAdditionalInfoFailureWarningLifecycle(t *testing.T) {
	c, err := cache.NewStringCache(setting.Cache{Adapter: "memory", Interval: 1})
	require.NoError(t, err)

	mockNow := time.Unix(1_700_000_000, 0)
	defer timeutil.MockSet(mockNow)()
	SetRequiredAdditionalInfoFetchFailureWarning(c, "Google Workspace")

	warning := GetRequiredAdditionalInfoFailureWarning(c)
	require.NotNil(t, warning)
	assert.Equal(t, "Google Workspace", warning.SourceName)
	assert.Equal(t, timeutil.TimeStamp(mockNow.Unix()), warning.LastFailedUnix)

	ClearRequiredAdditionalInfoFetchFailureWarning(c)
	assert.Nil(t, GetRequiredAdditionalInfoFailureWarning(c))
}

func TestRequiredAdditionalInfoFailureWarningThrottle(t *testing.T) {
	c, err := cache.NewStringCache(setting.Cache{Adapter: "memory", Interval: 1})
	require.NoError(t, err)

	first := time.Unix(1_700_000_000, 0)
	defer timeutil.MockSet(first)()
	SetRequiredAdditionalInfoFetchFailureWarning(c, "Google Workspace")
	initial := GetRequiredAdditionalInfoFailureWarning(c)
	require.NotNil(t, initial)

	// Within throttle window, keep previous timestamp to avoid cache churn.
	timeutil.MockSet(first.Add(30 * time.Second))
	SetRequiredAdditionalInfoFetchFailureWarning(c, "Google Workspace")
	throttled := GetRequiredAdditionalInfoFailureWarning(c)
	require.NotNil(t, throttled)
	assert.Equal(t, initial.LastFailedUnix, throttled.LastFailedUnix)

	// After throttle window, timestamp is refreshed.
	timeutil.MockSet(first.Add(61 * time.Second))
	SetRequiredAdditionalInfoFetchFailureWarning(c, "Google Workspace")
	refreshed := GetRequiredAdditionalInfoFailureWarning(c)
	require.NotNil(t, refreshed)
	assert.Equal(t, timeutil.TimeStamp(first.Add(61*time.Second).Unix()), refreshed.LastFailedUnix)
}
