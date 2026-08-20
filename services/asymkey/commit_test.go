// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommitWithGPGSignatureInstanceSSHKey(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	sshPubKeyPath := filepath.Join(t.TempDir(), "gitea.pub")
	require.NoError(t, os.WriteFile(sshPubKeyPath, []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILpPrMXSg9qTx04jPNPWRcHsutyxWjThIpzcaO68yWVn\n"), 0o600))

	defer test.MockVariableValue(&setting.Repository.Signing.SigningFormat, git.SigningKeyFormatSSH)()
	defer test.MockVariableValue(&setting.Repository.Signing.SigningKey, sshPubKeyPath)()
	defer test.MockVariableValue(&setting.Repository.Signing.SigningName, "gitea")()
	defer test.MockVariableValue(&setting.Repository.Signing.SigningEmail, "gitea@fake.local")()

	commit, err := git.CommitFromReader(git.Sha1ObjectFormat.EmptyObjectID(), strings.NewReader(`tree f1a6cb52b2d16773290cefe49ad0684b50a4f930
author silverwind <me@silverwind.io> 1563741793 +0200
committer silverwind <me@silverwind.io> 1563741793 +0200
gpgsig -----BEGIN PGP SIGNATURE-----
`+" "+`
 iQIzBAABCAAdFiEEWPb2jX6FS2mqyJRQLmK0HJOGlEMFAl00zmEACgkQLmK0HJOG
 lEMDFBAAhQKKqLD1VICygJMEB8t1gBmNLgvziOLfpX4KPWdPtBk3v/QJ7OrfMrVK
 xlC4ZZyx6yMm1Q7GzmuWykmZQJ9HMaHJ49KAbh5MMjjV/+OoQw9coIdo8nagRUld
 vX8QHzNZ6Agx77xHuDJZgdHKpQK3TrMDsxzoYYMvlqoLJIDXE1Sp7KYNy12nhdRg
 R6NXNmW8oMZuxglkmUwayMiPS+N4zNYqv0CXYzlEqCOgq9MJUcAMHt+KpiST+sm6
 FWkJ9D+biNPyQ9QKf1AE4BdZia4lHfPYU/C/DEL/a5xQuuop/zMQZoGaIA4p2zGQ
 /maqYxEIM/yRBQpT1jlODKPJrMEgx7SgY2hRU47YZ4fj6350fb6fNBtiiMAfJbjL
 S3Gh85E9fm3hJaNSPKAaJFYL1Ya2svuWfgHj677C56UcmYis7fhiiy1aJuYdHnSm
 sD53z/f0J+We4VZjY+pidvA9BGZPFVdR3wd3xGs8/oH6UWaLJAMGkLG6dDb3qDLm
 1LFZwsX8sdD32i1SiWanYQYSYMyFWr0awi4xdoMtYCL7uKBYtwtPyvq3cj4IrJlb
 mfeFhT57UbE4qukTDIQ0Y0WM40UYRTakRaDY7ubhXgLgx09Cnp9XTVMsHgT6j9/i
 1pxsB104XLWjQHTjr1JtiaBQEwFh9r2OKTcpvaLcbNtYpo7CzOs=
 =FRsO
 -----END PGP SIGNATURE-----

empty commit
`))
	require.NoError(t, err)

	lc, cleanup := test.NewLogChecker(log.DEFAULT)
	defer cleanup()
	lc.Filter("Error getting default signing key").StopMark("instance ssh key check done")

	ret := parseCommitWithGPGSignature(t.Context(), commit, &user_model.User{ID: 2, Name: "user2", Email: "user2@example.com"})
	require.NotNil(t, ret)
	assert.False(t, ret.Verified)
	assert.Equal(t, asymkey_model.NoKeyFound, ret.Reason)

	// depending on whether gpg exports an empty key or fails outright, using the SSH key here would either
	// report a bogus hash error above or log an export failure, so neither may happen
	log.Info("instance ssh key check done")
	triedGPGExport, stopped := lc.Check(5 * time.Second)
	assert.True(t, stopped)
	assert.False(t, triedGPGExport[0])
}

func TestParseCommitWithSSHSignature(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Here we only need to do some tests that "tests/integration/gpg_ssh_git_test.go" doesn't cover

	// -----BEGIN OPENSSH PRIVATE KEY-----
	// b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
	// QyNTUxOQAAACC6T6zF0oPak8dOIzzT1kXB7LrcsVo04SKc3GjuvMllZwAAAJgy08upMtPL
	// qQAAAAtzc2gtZWQyNTUxOQAAACC6T6zF0oPak8dOIzzT1kXB7LrcsVo04SKc3GjuvMllZw
	// AAAEDWqPHTH51xb4hy1y1f1VeWL/2A9Q0b6atOyv5fx8x5prpPrMXSg9qTx04jPNPWRcHs
	// utyxWjThIpzcaO68yWVnAAAAEXVzZXIyQGV4YW1wbGUuY29tAQIDBA==
	// -----END OPENSSH PRIVATE KEY-----
	sshPubKey, err := asymkey_model.AddPublicKey(t.Context(), 999, "user-ssh-key-any-name", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILpPrMXSg9qTx04jPNPWRcHsutyxWjThIpzcaO68yWVn", 0, false)
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).ID(sshPubKey.ID).Cols("verified").Update(&asymkey_model.PublicKey{Verified: true})
	require.NoError(t, err)

	t.Run("UserSSHKey", func(t *testing.T) {
		commit, err := git.CommitFromReader(git.Sha1ObjectFormat.EmptyObjectID(), strings.NewReader(`tree a3b1fad553e0f9a2b4a58327bebde36c7da75aa2
author user2 <user2@example.com> 1752194028 -0700
committer user2 <user2@example.com> 1752194028 -0700
gpgsig -----BEGIN SSH SIGNATURE-----
 U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAguk+sxdKD2pPHTiM809ZFwey63L
 FaNOEinNxo7rzJZWcAAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
 AAAAQBfX+6mcKZBnXckwHcBFqRuXMD3vTKi1yv5wgrqIxTyr2LWB97xxmO92cvjsr0POQ2
 2YA7mQS510Cg2s1uU1XAk=
 -----END SSH SIGNATURE-----

init project
`))
		require.NoError(t, err)

		// the committingUser is guaranteed by the caller, parseCommitWithSSHSignature doesn't do any more checks
		committingUser := &user_model.User{ID: 999, Name: "user-x"}
		ret := parseCommitWithSSHSignature(t.Context(), commit, committingUser)
		require.NotNil(t, ret)
		assert.True(t, ret.Verified)
		assert.Equal(t, committingUser.Name+" / "+sshPubKey.Fingerprint, ret.Reason)
		assert.False(t, ret.Warning)
		assert.Equal(t, committingUser, ret.SigningUser)
		assert.Equal(t, committingUser, ret.CommittingUser)
		assert.Equal(t, sshPubKey.ID, ret.SigningSSHKey.ID)
	})

	t.Run("TrustedSSHKey", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Repository.Signing.SigningName, "gitea")()
		defer test.MockVariableValue(&setting.Repository.Signing.SigningEmail, "gitea@fake.local")()
		defer test.MockVariableValue(&setting.Repository.Signing.TrustedSSHKeys, []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH6Y4idVaW3E+bLw1uqoAfJD7o5Siu+HqS51E9oQLPE9"})()

		commit, err := git.CommitFromReader(git.Sha1ObjectFormat.EmptyObjectID(), strings.NewReader(`tree 9a93ffa76e8b72bdb6431910b3a506fa2b39f42e
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
}
