// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestAddLdapSSHPublicKeys(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	s := &auth.Source{ID: 1}

	const (
		rsaKey     = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM="
		ed25519Key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf"
	)
	for i, kase := range []struct {
		keyString   string
		number      int
		keyContents []string
	}{
		{rsaKey + " nocomment\n", 1, []string{rsaKey}},
		{rsaKey + " nocomment\n" + ed25519Key + " nocomment", 2, []string{rsaKey, ed25519Key}},
		{rsaKey + " nocomment\n# comment asmdna,ndp\n" + ed25519Key + " nocomment", 2, []string{rsaKey, ed25519Key}},
		{rsaKey + " nocomment\n382488320jasdj1lasmva/vasodifipi4193-fksma.cm\n" + ed25519Key + " nocomment", 2, []string{rsaKey, ed25519Key}},
	} {
		s.ID = int64(i) + 20
		asymkey_model.AddPublicKeysBySource(t.Context(), user, s, []string{kase.keyString}, false)
		keys, err := db.Find[asymkey_model.PublicKey](t.Context(), asymkey_model.FindPublicKeyOptions{
			OwnerID:       user.ID,
			LoginSourceID: s.ID,
		})
		assert.NoError(t, err)
		if err != nil {
			continue
		}
		assert.Len(t, keys, kase.number)

		for _, key := range keys {
			assert.Contains(t, kase.keyContents, key.Content)
		}
		for _, key := range keys {
			DeletePublicKey(t.Context(), user, key.ID)
		}
	}
}
