// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"testing"

	"gitea.dev/modules/optional"
	"gitea.dev/services/auth/source/oauth2"
	user_service "gitea.dev/services/user"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldSyncFromGroupClaim(t *testing.T) {
	t.Run("google claim missing", func(t *testing.T) {
		source := &oauth2.Source{
			Provider:       "gplus",
			GroupClaimName: "groups",
		}
		user := &goth.User{
			RawData: map[string]any{},
		}
		assert.False(t, shouldSyncFromGroupClaim(source, user))
	})

	t.Run("google claim present and empty", func(t *testing.T) {
		source := &oauth2.Source{
			Provider:       "gplus",
			GroupClaimName: "groups",
		}
		user := &goth.User{
			RawData: map[string]any{
				"groups": []string{},
			},
		}
		assert.True(t, shouldSyncFromGroupClaim(source, user))
	})

	t.Run("non google provider keeps old behavior", func(t *testing.T) {
		source := &oauth2.Source{
			Provider:       "openidConnect",
			GroupClaimName: "groups",
		}
		user := &goth.User{
			RawData: map[string]any{},
		}
		assert.True(t, shouldSyncFromGroupClaim(source, user))
	})
}

func TestGetUserAdminAndRestrictedFromGroupClaims_GoogleMissingClaim(t *testing.T) {
	source := &oauth2.Source{
		Provider:        "gplus",
		GroupClaimName:  "groups",
		AdminGroup:      "g-admins@example.com",
		RestrictedGroup: "g-restricted@example.com",
	}
	user := &goth.User{
		RawData: map[string]any{},
	}

	isAdmin, isRestricted := getUserAdminAndRestrictedFromGroupClaims(source, user)

	assert.False(t, isAdmin.Has())
	assert.Equal(t, optional.None[bool](), isRestricted)
}

func TestGetUserAdminAndRestrictedFromGroupClaims_GoogleEmptyClaim(t *testing.T) {
	source := &oauth2.Source{
		Provider:        "gplus",
		GroupClaimName:  "groups",
		AdminGroup:      "g-admins@example.com",
		RestrictedGroup: "g-restricted@example.com",
	}
	user := &goth.User{
		RawData: map[string]any{
			"groups": []string{},
		},
	}

	isAdmin, isRestricted := getUserAdminAndRestrictedFromGroupClaims(source, user)

	require.True(t, isAdmin.Has())
	assert.Equal(t, user_service.UpdateOptionFieldFromSync(false), isAdmin)
	assert.Equal(t, optional.Some(false), isRestricted)
}

func TestGetUserAdminAndRestrictedFromGroupClaims_NonGoogleMissingClaim(t *testing.T) {
	source := &oauth2.Source{
		Provider:        "openidConnect",
		GroupClaimName:  "groups",
		AdminGroup:      "g-admins@example.com",
		RestrictedGroup: "g-restricted@example.com",
	}
	user := &goth.User{
		RawData: map[string]any{},
	}

	isAdmin, isRestricted := getUserAdminAndRestrictedFromGroupClaims(source, user)

	require.True(t, isAdmin.Has())
	assert.Equal(t, user_service.UpdateOptionFieldFromSync(false), isAdmin)
	assert.Equal(t, optional.Some(false), isRestricted)
}
