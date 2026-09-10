// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisableTwoFactorCommand(t *testing.T) {
	ctx := t.Context()

	defer func() {
		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.User{}, &auth_model.TwoFactor{}, &auth_model.WebAuthnCredential{}))
	}()

	t.Run("disable TOTP and WebAuthn", func(t *testing.T) {
		require.NoError(t, microcmdUserCreate().Run(ctx, []string{"create", "--username", "tfuser", "--email", "tfuser@gitea.local", "--random-password"}))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "tfuser"})

		// Enroll TOTP.
		tf := &auth_model.TwoFactor{UID: user.ID}
		require.NoError(t, tf.SetSecret("test-secret"))
		_, err := tf.GenerateScratchToken()
		require.NoError(t, err)
		require.NoError(t, auth_model.NewTwoFactor(ctx, tf))

		// Register a WebAuthn credential.
		_, err = auth_model.CreateCredential(ctx, user.ID, "test-key", &webauthn.Credential{ID: []byte("test-cred-id")})
		require.NoError(t, err)

		has, err := auth_model.HasTwoFactorOrWebAuthn(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, has)

		require.NoError(t, microcmdUserDisableTwoFactor().Run(ctx, []string{"disable-2fa", "--username", "tfuser"}))

		// Both factors must be gone afterwards.
		has, err = auth_model.HasTwoFactorOrWebAuthn(ctx, user.ID)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("disable by id", func(t *testing.T) {
		require.NoError(t, microcmdUserCreate().Run(ctx, []string{"create", "--username", "iduser", "--email", "iduser@gitea.local", "--random-password"}))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "iduser"})

		tf := &auth_model.TwoFactor{UID: user.ID}
		require.NoError(t, tf.SetSecret("test-secret"))
		require.NoError(t, auth_model.NewTwoFactor(ctx, tf))

		require.NoError(t, microcmdUserDisableTwoFactor().Run(ctx, []string{"disable-2fa", "--id", strconv.FormatInt(user.ID, 10)}))

		has, err := auth_model.HasTwoFactorOrWebAuthn(ctx, user.ID)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("no enrollment is a no-op", func(t *testing.T) {
		require.NoError(t, microcmdUserCreate().Run(ctx, []string{"create", "--username", "plainuser", "--email", "plainuser@gitea.local", "--random-password"}))
		require.NoError(t, microcmdUserDisableTwoFactor().Run(ctx, []string{"disable-2fa", "--username", "plainuser"}))
	})

	t.Run("id and username must match when both given", func(t *testing.T) {
		require.NoError(t, microcmdUserCreate().Run(ctx, []string{"create", "--username", "matchuser", "--email", "matchuser@gitea.local", "--random-password"}))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "matchuser"})
		id := strconv.FormatInt(user.ID, 10)

		// Matching id + username is accepted.
		require.NoError(t, microcmdUserDisableTwoFactor().Run(ctx, []string{"disable-2fa", "--id", id, "--username", "matchuser"}))

		// Mismatched id + username is rejected.
		cmd := microcmdUserDisableTwoFactor()
		cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
		err := cmd.Run(ctx, []string{"disable-2fa", "--id", id, "--username", "someotheruser"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not match the provided username")
	})

	t.Run("failure cases", func(t *testing.T) {
		testCases := []struct {
			name        string
			args        []string
			expectedErr string
		}{
			{
				name:        "user does not exist",
				args:        []string{"disable-2fa", "--username", "nonexistentuser"},
				expectedErr: "user does not exist",
			},
			{
				name:        "neither id nor username",
				args:        []string{"disable-2fa"},
				expectedErr: "either --id or --username must be provided",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cmd := microcmdUserDisableTwoFactor()
				cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
				err := cmd.Run(ctx, tc.args)
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			})
		}
	})
}
