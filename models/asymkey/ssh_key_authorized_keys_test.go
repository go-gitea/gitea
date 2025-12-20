// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestWriteAuthorizedStringForKey(t *testing.T) {
	defer test.MockVariableValue(&setting.AppPath, "/tmp/gitea")()
	defer test.MockVariableValue(&setting.CustomConf, "/tmp/app.ini")()
	writeKey := func(t *testing.T, key *PublicKey) (bool, string, error) {
		sb := &strings.Builder{}
		valid, err := writeAuthorizedStringForKey(key, sb)
		return valid, sb.String(), err
	}
	const validKeyContent = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf`

	testValid := func(t *testing.T, key *PublicKey, expected string) {
		valid, content, err := writeKey(t, key)
		assert.True(t, valid)
		assert.Equal(t, expected, content)
		assert.NoError(t, err)
	}

	testInvalid := func(t *testing.T, key *PublicKey) {
		valid, content, err := writeKey(t, key)
		assert.False(t, valid)
		assert.Empty(t, content)
		assert.Error(t, err)
	}

	t.Run("PublicKey", func(t *testing.T) {
		testValid(t, &PublicKey{
			OwnerID: 123,
			Content: validKeyContent + " any-comment",
			Type:    KeyTypeUser,
		}, `# gitea public key
command="/tmp/gitea --config=/tmp/app.ini serv key-0",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty,no-user-rc,restrict ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf user-123
`)
	})

	t.Run("PublicKeyWithNewLine", func(t *testing.T) {
		testValid(t, &PublicKey{
			OwnerID: 123,
			Content: validKeyContent + "\nany-more", // the new line should be ignored
			Type:    KeyTypeUser,
		}, `# gitea public key
command="/tmp/gitea --config=/tmp/app.ini serv key-0",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty,no-user-rc,restrict ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf user-123
`)
	})

	t.Run("PublicKeyInvalid", func(t *testing.T) {
		testInvalid(t, &PublicKey{
			OwnerID: 123,
			Content: validKeyContent + "any-more",
			Type:    KeyTypeUser,
		})
	})

	t.Run("Principal", func(t *testing.T) {
		testValid(t, &PublicKey{
			OwnerID: 123,
			Content: "any-content",
			Type:    KeyTypePrincipal,
		}, `# gitea public key
command="/tmp/gitea --config=/tmp/app.ini serv key-0",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty,no-user-rc,restrict any-content # user-123
`)
	})

	t.Run("PrincipalInvalid", func(t *testing.T) {
		testInvalid(t, &PublicKey{
			OwnerID: 123,
			Content: "a b",
			Type:    KeyTypePrincipal,
		})
		testInvalid(t, &PublicKey{
			OwnerID: 123,
			Content: "a\nb",
			Type:    KeyTypePrincipal,
		})
	})
}
