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

type mockBotSource struct {
	auth_model.ConfigBase
}

func (s *mockBotSource) FromDB(bs []byte) error { return nil }
func (s *mockBotSource) ToDB() ([]byte, error)  { return []byte("{}"), nil }

func (s *mockBotSource) Authenticate(ctx context.Context, _ *user_model.User, login, _ string) (*user_model.User, error) {
	return user_model.GetUserByName(ctx, login)
}

const mockBotSourceType auth_model.Type = 100

func init() {
	auth_model.RegisterTypeConfig(mockBotSourceType, &mockBotSource{})
}

func TestUserSignIn_BotCannotSignIn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	bot := &user_model.User{Name: "test-bot", Email: "test-bot@example.com", Type: user_model.UserTypeBot, IsActive: true}
	require.NoError(t, user_model.AdminCreateUser(t.Context(), bot, &user_model.Meta{}))
	require.NoError(t, db.Insert(t.Context(), &auth_model.Source{
		Type:     mockBotSourceType,
		Name:     "mock-bot-source",
		IsActive: true,
		Cfg:      &mockBotSource{},
	}))

	_, _, err := UserSignIn(t.Context(), "test-bot", "")
	assert.ErrorAs(t, err, &user_model.ErrUserNotExist{})
}
