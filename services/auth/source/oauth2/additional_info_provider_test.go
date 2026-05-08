// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
