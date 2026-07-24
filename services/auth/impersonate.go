// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
)

func ImpersonateUser(sess SessionStore, u *user_model.User) error {
	// TODO: in the future, we need to process all sessions keys, but the session store doesn't have the ability to list keys
	// So we need to refactor all "Session.Get" to use consts, then we can enumerate the pre-defined keys.
	backupKeys := []string{session.KeyUID, session.KeyUserHasTwoFactorAuth}
	backup := map[string]any{}
	for _, key := range backupKeys {
		v := sess.Get(key)
		if v != nil {
			backup[key] = v
		}
	}
	err := sess.Set(session.KeyImpersonatorData, backup)
	if err != nil {
		return fmt.Errorf("set impersonator data: %w", err)
	}

	ClearSessionKeysForSignIn(sess)
	data := map[string]any{}
	data[session.KeyUID] = u.ID
	data[session.KeyUserHasTwoFactorAuth] = true // since we are impersonating, we don't want to require 2FA for the impersonated user
	for k, v := range data {
		if err = sess.Set(k, v); err != nil {
			return fmt.Errorf("set session data: %w", err)
		}
	}
	return sess.Release()
}

func ExitImpersonatedUser(sess SessionStore) (bool, error) {
	impersonatorData, ok := sess.Get(session.KeyImpersonatorData).(map[string]any)
	if !ok {
		return false, nil
	}
	err := sess.Delete(session.KeyImpersonatorData)
	if err != nil {
		return false, fmt.Errorf("delete impersonator data: %w", err)
	}

	ClearSessionKeysForSignIn(sess)
	for k, v := range impersonatorData {
		if err = sess.Set(k, v); err != nil {
			return false, fmt.Errorf("set impersonator data: %w", err)
		}
	}
	return true, sess.Release()
}
