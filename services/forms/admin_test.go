// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"testing"

	"gitea.dev/modules/validation"

	"github.com/stretchr/testify/assert"
)

func TestAdminCreateUserFormUserType(t *testing.T) {
	for _, userType := range []string{"", "individual", "bot"} {
		form := &AdminCreateUserForm{
			LoginType: "local",
			UserName:  "user",
			UserType:  userType,
			Email:     "user@example.com",
		}
		assert.Empty(t, validation.Binder().Validate(t.Context(), form))
	}

	form := &AdminCreateUserForm{
		LoginType: "local",
		UserName:  "user",
		UserType:  "invalid",
		Email:     "user@example.com",
	}
	assert.NotEmpty(t, validation.Binder().Validate(t.Context(), form))
}
