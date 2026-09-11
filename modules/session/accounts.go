// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"slices"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
)

// MaxSignedInAccounts bounds how many accounts one session can hold.
const MaxSignedInAccounts = 5

// KVStore is the subset of a session store needed to manage the signed-in account list.
type KVStore interface {
	Get(key any) any
	Set(key, value any) error
	Delete(key any) error
}

// SignedInAccount is one account which completed authentication within this session.
type SignedInAccount struct {
	UID              int64  `json:"uid"`
	HasTwoFactorAuth bool   `json:"tfa,omitempty"`
	SignInMethod     string `json:"method,omitempty"`
}

// GetSignedInAccounts returns the accounts authenticated in this session, most recent first.
func GetSignedInAccounts(sess KVStore) []SignedInAccount {
	// the value is stored as JSON because session data is gob-encoded and gob cannot encode unregistered types
	s, ok := sess.Get(KeySignedInAccounts).(string)
	if !ok || s == "" {
		return nil
	}
	var accounts []SignedInAccount
	if err := json.Unmarshal([]byte(s), &accounts); err != nil {
		log.Error("Unable to decode signed-in accounts from session: %v", err)
		return nil // tolerate a broken value instead of locking the user out
	}
	return accounts
}

// SetSignedInAccounts replaces the signed-in account list of this session.
func SetSignedInAccounts(sess KVStore, accounts []SignedInAccount) error {
	if len(accounts) == 0 {
		return sess.Delete(KeySignedInAccounts)
	}
	b, err := json.Marshal(accounts)
	if err != nil {
		return err
	}
	return sess.Set(KeySignedInAccounts, string(b))
}

// AddSignedInAccount adds or refreshes an account and moves it to the front.
func AddSignedInAccount(sess KVStore, acc SignedInAccount) error {
	accounts := slices.DeleteFunc(GetSignedInAccounts(sess), func(a SignedInAccount) bool { return a.UID == acc.UID })
	accounts = append([]SignedInAccount{acc}, accounts...)
	if len(accounts) > MaxSignedInAccounts {
		accounts = accounts[:MaxSignedInAccounts]
	}
	return SetSignedInAccounts(sess, accounts)
}

// RemoveSignedInAccount drops an account and returns the remaining ones.
func RemoveSignedInAccount(sess KVStore, uid int64) ([]SignedInAccount, error) {
	accounts := slices.DeleteFunc(GetSignedInAccounts(sess), func(a SignedInAccount) bool { return a.UID == uid })
	return accounts, SetSignedInAccounts(sess, accounts)
}

// FindSignedInAccount looks up an account by user id.
func FindSignedInAccount(accounts []SignedInAccount, uid int64) (SignedInAccount, bool) {
	i := slices.IndexFunc(accounts, func(a SignedInAccount) bool { return a.UID == uid })
	if i < 0 {
		return SignedInAccount{}, false
	}
	return accounts[i], true
}
