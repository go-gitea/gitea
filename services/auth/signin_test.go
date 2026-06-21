// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBotSource reproduces the behaviour of external sources (e.g. LDAP) that
// resolve an already existing local user by name without checking its type.
type mockBotSource struct {
	auth_model.ConfigBase
}

func (s *mockBotSource) FromDB(bs []byte) error { return nil }
func (s *mockBotSource) ToDB() ([]byte, error)  { return []byte("{}"), nil }

func (s *mockBotSource) Authenticate(ctx context.Context, _ *user_model.User, login, _ string) (*user_model.User, error) {
	return user_model.GetUserByName(ctx, login)
}

// mockBotSourceType is a test-only auth source type, kept out of the real enum range.
const mockBotSourceType auth_model.Type = 100

func init() {
	auth_model.RegisterTypeConfig(mockBotSourceType, &mockBotSource{})
}

func TestUserSignIn_BotCannotSignIn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	bot := &user_model.User{
		Name:               "test-bot",
		Email:              "test-bot@example.com",
		Type:               user_model.UserTypeBot,
		MustChangePassword: false,
		IsActive:           true,
	}
	require.NoError(t, user_model.AdminCreateUser(t.Context(), bot, &user_model.Meta{}))

	// register an active external source that would otherwise hand back the bot user
	require.NoError(t, db.Insert(t.Context(), &auth_model.Source{
		Type:     mockBotSourceType,
		Name:     "mock-bot-source",
		IsActive: true,
		Cfg:      &mockBotSource{},
	}))

	// a bot has no password and must not be able to sign in interactively, neither
	// via the local source nor via the external source fallback loop
	_, _, err := UserSignIn(t.Context(), "test-bot", "")
	assert.ErrorAs(t, err, &user_model.ErrUserNotExist{})
}
