// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"testing"

	"gitea.com/go-chi/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore map[any]any

func (f fakeStore) Get(key any) any { return f[key] }

func (f fakeStore) Set(key, value any) error { f[key] = value; return nil }

func (f fakeStore) Delete(key any) error { delete(f, key); return nil }

func TestSignedInAccounts(t *testing.T) {
	t.Run("AddMovesToFrontAndDedups", func(t *testing.T) {
		sess := fakeStore{}
		require.NoError(t, AddSignedInAccount(sess, SignedInAccount{UID: 1}))
		require.NoError(t, AddSignedInAccount(sess, SignedInAccount{UID: 2}))
		require.NoError(t, AddSignedInAccount(sess, SignedInAccount{UID: 1, AuthTokenID: "t1"}))

		accounts := GetSignedInAccounts(sess)
		assert.Equal(t, []SignedInAccount{{UID: 1, AuthTokenID: "t1"}, {UID: 2}}, accounts)
	})

	t.Run("CapEvictsOldest", func(t *testing.T) {
		sess := fakeStore{}
		for i := range int64(MaxSignedInAccounts + 2) {
			require.NoError(t, AddSignedInAccount(sess, SignedInAccount{UID: i + 1}))
		}
		accounts := GetSignedInAccounts(sess)
		require.Len(t, accounts, MaxSignedInAccounts)
		assert.Equal(t, int64(MaxSignedInAccounts+2), accounts[0].UID)
		_, ok := FindSignedInAccount(accounts, 1)
		assert.False(t, ok)
	})

	t.Run("Remove", func(t *testing.T) {
		sess := fakeStore{}
		require.NoError(t, AddSignedInAccount(sess, SignedInAccount{UID: 1}))
		require.NoError(t, AddSignedInAccount(sess, SignedInAccount{UID: 2}))

		remaining, err := RemoveSignedInAccount(sess, 2)
		require.NoError(t, err)
		assert.Equal(t, []SignedInAccount{{UID: 1}}, remaining)

		remaining, err = RemoveSignedInAccount(sess, 1)
		require.NoError(t, err)
		assert.Empty(t, remaining)
		assert.Nil(t, sess.Get(KeySignedInAccounts))
	})

	t.Run("TolerateBrokenValue", func(t *testing.T) {
		assert.Nil(t, GetSignedInAccounts(fakeStore{KeySignedInAccounts: 42}))
		assert.Nil(t, GetSignedInAccounts(fakeStore{KeySignedInAccounts: "not json"}))
	})

	t.Run("StoredValueIsGobEncodable", func(t *testing.T) {
		// session data is gob-encoded and gob cannot encode unregistered types, so the list must be a string
		sess := fakeStore{}
		require.NoError(t, AddSignedInAccount(sess, SignedInAccount{UID: 1, SignInMethod: SignInMethodOAuth2}))
		_, err := session.EncodeGob(map[any]any(sess))
		require.NoError(t, err)
	})
}
