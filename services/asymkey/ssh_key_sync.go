// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"

	"golang.org/x/crypto/ssh"
)

// deleteKeysMarkedForDeletion returns true if ssh keys needs update
func deleteKeysMarkedForDeletion(ctx context.Context, keys []string) (bool, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (bool, error) {
		// Delete keys marked for deletion
		var sshKeysNeedUpdate bool
		for _, sshKeyToDelete := range keys {
			key, err := asymkey_model.SearchPublicKeyForSSH(ctx, sshKeyToDelete)
			if err != nil {
				log.Error("SearchPublicKeyForSSH: %v", err)
				continue
			}
			if _, err = db.DeleteByID[asymkey_model.PublicKey](ctx, key.ID); err != nil {
				log.Error("DeleteByID[PublicKey]: %v", err)
				continue
			}
			audit.RecordAs(ctx, user_model.NewAuthSourceUser(), audit_model.UserKeySSHRemove, audit.ScopeFromUserID(ctx, key.OwnerID), "fingerprint", key.Fingerprint)
			sshKeysNeedUpdate = true
		}

		return sshKeysNeedUpdate, nil
	})
}

// AddPublicKeysBySource add a users public keys. Returns true if there are changes.
func AddPublicKeysBySource(ctx context.Context, usr *user_model.User, s *auth_model.Source, sshPublicKeys []string, verified bool) bool {
	var sshKeysNeedUpdate bool
	for _, sshKey := range sshPublicKeys {
		var err error
		found := false
		keys := []byte(sshKey)
	loop:
		for len(keys) > 0 && err == nil {
			var out ssh.PublicKey
			// We ignore options as they are not relevant to Gitea
			out, _, _, keys, err = ssh.ParseAuthorizedKey(keys)
			if err != nil {
				break loop
			}
			found = true
			marshalled := string(ssh.MarshalAuthorizedKey(out))
			marshalled = marshalled[:len(marshalled)-1]
			sshKeyName := fmt.Sprintf("%s-%s", s.Name, ssh.FingerprintSHA256(out))

			key, addErr := asymkey_model.AddPublicKey(ctx, usr.ID, sshKeyName, marshalled, s.ID, verified)
			if addErr != nil {
				if asymkey_model.IsErrKeyAlreadyExist(addErr) {
					log.Trace("AddPublicKeysBySource[%s]: Public SSH Key %s already exists for user", sshKeyName, usr.Name)
				} else {
					log.Error("AddPublicKeysBySource[%s]: Error adding Public SSH Key for user %s: %v", sshKeyName, usr.Name, addErr)
				}
			} else {
				audit.RecordAs(ctx, user_model.NewAuthSourceUser(), audit_model.UserKeySSHAdd, usr, "fingerprint", key.Fingerprint)
				log.Trace("AddPublicKeysBySource[%s]: Added Public SSH Key for user %s", sshKeyName, usr.Name)
				sshKeysNeedUpdate = true
			}
		}
		if !found && err != nil {
			log.Warn("AddPublicKeysBySource[%s]: Skipping invalid Public SSH Key for user %s: %v", s.Name, usr.Name, sshKey)
		}
	}
	return sshKeysNeedUpdate
}

// SynchronizePublicKeys updates a user's public keys. Returns true if there are changes.
func SynchronizePublicKeys(ctx context.Context, usr *user_model.User, s *auth_model.Source, sshPublicKeys []string, verified bool) bool {
	var sshKeysNeedUpdate bool

	log.Trace("synchronizePublicKeys[%s]: Handling Public SSH Key synchronization for user %s", s.Name, usr.Name)

	// Get Public Keys from DB with the current auth source
	var giteaKeys []string
	keys, err := db.Find[asymkey_model.PublicKey](ctx, asymkey_model.FindPublicKeyOptions{
		OwnerID:       usr.ID,
		LoginSourceID: s.ID,
	})
	if err != nil {
		log.Error("synchronizePublicKeys[%s]: Error listing Public SSH Keys for user %s: %v", s.Name, usr.Name, err)
	}

	for _, v := range keys {
		giteaKeys = append(giteaKeys, v.OmitEmail())
	}

	// Process the provided keys to remove duplicates and name part
	var providedKeys []string
	for _, v := range sshPublicKeys {
		sshKeySplit := strings.Split(v, " ")
		if len(sshKeySplit) > 1 {
			key := strings.Join(sshKeySplit[:2], " ")
			if !util.SliceContainsString(providedKeys, key) {
				providedKeys = append(providedKeys, key)
			}
		}
	}

	// Check if Public Key sync is needed
	if util.SliceSortedEqual(giteaKeys, providedKeys) {
		log.Trace("synchronizePublicKeys[%s]: Public Keys are already in sync for %s (Source:%v/DB:%v)", s.Name, usr.Name, len(providedKeys), len(giteaKeys))
		return false
	}
	log.Trace("synchronizePublicKeys[%s]: Public Key needs update for user %s (Source:%v/DB:%v)", s.Name, usr.Name, len(providedKeys), len(giteaKeys))

	// Add new Public SSH Keys that doesn't already exist in DB
	var newKeys []string
	for _, key := range providedKeys {
		if !util.SliceContainsString(giteaKeys, key) {
			newKeys = append(newKeys, key)
		}
	}
	if AddPublicKeysBySource(ctx, usr, s, newKeys, verified) {
		sshKeysNeedUpdate = true
	}

	// Mark keys from DB that no longer exist in the source for deletion
	var giteaKeysToDelete []string
	for _, giteaKey := range giteaKeys {
		if !util.SliceContainsString(providedKeys, giteaKey) {
			log.Trace("synchronizePublicKeys[%s]: Marking Public SSH Key for deletion for user %s: %v", s.Name, usr.Name, giteaKey)
			giteaKeysToDelete = append(giteaKeysToDelete, giteaKey)
		}
	}

	// Delete keys from DB that no longer exist in the source
	needUpd, err := deleteKeysMarkedForDeletion(ctx, giteaKeysToDelete)
	if err != nil {
		log.Error("synchronizePublicKeys[%s]: Error deleting Public Keys marked for deletion for user %s: %v", s.Name, usr.Name, err)
	}
	if needUpd {
		sshKeysNeedUpdate = true
	}

	return sshKeysNeedUpdate
}
