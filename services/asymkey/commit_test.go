// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"fmt"
	"strings"
	"testing"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommitWithSSHSignature(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Here we only test the TrustedSSHKeys. The complete signing test is in tests/integration/gpg_ssh_git_test.go
	t.Run("TrustedSSHKey", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Repository.Signing.SigningName, "gitea")()
		defer test.MockVariableValue(&setting.Repository.Signing.SigningEmail, "gitea@fake.local")()
		defer test.MockVariableValue(&setting.Repository.Signing.TrustedSSHKeys, []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH6Y4idVaW3E+bLw1uqoAfJD7o5Siu+HqS51E9oQLPE9"})()

		commit, err := git.CommitFromReader(nil, git.Sha1ObjectFormat.EmptyObjectID(), strings.NewReader(`tree 9a93ffa76e8b72bdb6431910b3a506fa2b39f42e
author User Two <user2@example.com> 1749230009 +0200
committer User Two <user2@example.com> 1749230009 +0200
gpgsig -----BEGIN SSH SIGNATURE-----
 U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAgfpjiJ1VpbcT5svDW6qgB8kPujl
 KK74epLnUT2hAs8T0AAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
 AAAAQDX2t2iHuuLxEWHLJetYXKsgayv3c43r0pJNfAzdLN55Q65pC5M7rG6++gT2bxcpOu
 Y6EXbpLqia9sunEF3+LQY=
 -----END SSH SIGNATURE-----

Initial commit with signed file
`))
		require.NoError(t, err)
		committingUser := &user_model.User{
			ID:    2,
			Name:  "User Two",
			Email: "user2@example.com",
		}
		ret := parseCommitWithSSHSignature(t.Context(), commit, committingUser)
		require.NotNil(t, ret)
		assert.True(t, ret.Verified)
		assert.False(t, ret.Warning)
		assert.Equal(t, committingUser, ret.CommittingUser)
		if assert.NotNil(t, ret.SigningUser) {
			assert.Equal(t, "gitea", ret.SigningUser.Name)
			assert.Equal(t, "gitea@fake.local", ret.SigningUser.Email)
		}
	})

	t.Run("SSH_Sign_With_UserEmail", func(t *testing.T) {
		commit, err := git.CommitFromReader(nil, git.Sha1ObjectFormat.EmptyObjectID(), strings.NewReader(fmt.Sprintf(`tree a3b1fad553e0f9a2b4a58327bebde36c7da75aa2
author user2 <user2@example.com> 1752194028 -0700
committer user2 <user2@example.com> 1752194028 -0700
gpgsig -----BEGIN SSH SIGNATURE-----
 U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAguk+sxdKD2pPHTiM809ZFwey63L
 FaNOEinNxo7rzJZWcAAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
 AAAAQBfX+6mcKZBnXckwHcBFqRuXMD3vTKi1yv5wgrqIxTyr2LWB97xxmO92cvjsr0POQ2
 2YA7mQS510Cg2s1uU1XAk=
 -----END SSH SIGNATURE-----

init project
`)))
		require.NoError(t, err)
		committingUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		sshKey := unittest.AssertExistsAndLoadBean(t, &asymkey_model.PublicKey{ID: 2})

		ret := ParseCommitWithSSHSignature(t.Context(), commit, committingUser)
		require.NotNil(t, ret)
		assert.True(t, ret.Verified)
		assert.Equal(t, "user2 / SHA256:70+Mc6lGvBwTlXkxDqJwXZYqV6XzU7jDE06+lC+gocY", ret.Reason)
		assert.False(t, ret.Warning)
		assert.Equal(t, committingUser, ret.SigningUser)
		assert.Equal(t, committingUser, ret.CommittingUser)
		assert.Equal(t, sshKey, ret.SigningSSHKey)
	})

	t.Run("SSH_Sign_With_NoReplyEmailAddress", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.NoReplyAddress, "noreply.localhost")()

		commit, err := git.CommitFromReader(nil, git.Sha1ObjectFormat.EmptyObjectID(), strings.NewReader(fmt.Sprintf(`tree a3b1fad553e0f9a2b4a58327bebde36c7da75aa2
author user2 <user2@%s> 1752193378 -0700
committer user2 <user2@%s> 1752193378 -0700
gpgsig -----BEGIN SSH SIGNATURE-----
 U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAguk+sxdKD2pPHTiM809ZFwey63L
 FaNOEinNxo7rzJZWcAAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
 AAAAQEbiBh8Ad9JVk4lGUsczwI+2ISaWvzBU+Y/6Lbh3F1sTRF8Rz1ZvVkYup+dVpOLswx
 1tBhGYsG8gKwvqGTWQYwI=
 -----END SSH SIGNATURE-----

init project
`, setting.Service.NoReplyAddress, setting.Service.NoReplyAddress)))
		require.NoError(t, err)
		committingUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		sshKey := unittest.AssertExistsAndLoadBean(t, &asymkey_model.PublicKey{ID: 2})

		ret := ParseCommitWithSSHSignature(t.Context(), commit, committingUser)
		require.NotNil(t, ret)
		assert.True(t, ret.Verified)
		assert.Equal(t, "user2 / SHA256:70+Mc6lGvBwTlXkxDqJwXZYqV6XzU7jDE06+lC+gocY", ret.Reason)
		assert.False(t, ret.Warning)
		assert.Equal(t, committingUser, ret.SigningUser)
		assert.Equal(t, committingUser, ret.CommittingUser)
		assert.Equal(t, sshKey, ret.SigningSSHKey)
	})
}
