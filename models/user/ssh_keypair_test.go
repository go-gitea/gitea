// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"strings"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestSSHKeypair(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	_, err := user_model.GetSSHKeypairByOwner(t.Context(), 2)
	assert.ErrorIs(t, err, util.ErrNotExist)

	created, err := user_model.CreateSSHKeypair(t.Context(), 2)
	require.NoError(t, err)
	stored, err := user_model.GetSSHKeypairByOwner(t.Context(), 2)
	require.NoError(t, err)
	assert.Equal(t, created, stored)

	privateKey, err := stored.PrivateKey()
	require.NoError(t, err)
	signer, err := ssh.ParsePrivateKey(privateKey)
	require.NoError(t, err)
	assert.Equal(t, stored.PublicKey, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))))

	regenerated, err := user_model.CreateSSHKeypair(t.Context(), 2)
	require.NoError(t, err)
	assert.NotEqual(t, created.PublicKey, regenerated.PublicKey)
}
