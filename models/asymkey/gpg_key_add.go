// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package asymkey

import (
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"

	"github.com/keybase/go-crypto/openpgp"
)

//   __________________  ________   ____  __.
//  /  _____/\______   \/  _____/  |    |/ _|____ ___.__.
// /   \  ___ |     ___/   \  ___  |      <_/ __ <   |  |
// \    \_\  \|    |   \    \_\  \ |    |  \  ___/\___  |
//  \______  /|____|    \______  / |____|__ \___  > ____|
//         \/                  \/          \/   \/\/
//    _____       .___  .___
//   /  _  \    __| _/__| _/
//	/  /_\  \  / __ |/ __ |
// /    |    \/ /_/ / /_/ |
// \____|__  /\____ \____ |
//         \/      \/    \/

// This file contains functions relating to adding GPG Keys

// addGPGKey add key, import and subkeys to database
func addGPGKey(e db.Engine, key *GPGKey, content string) (err error) {
	// Add GPGKeyImport
	if _, err = e.Insert(GPGKeyImport{
		KeyID:   key.KeyID,
		Content: content,
	}); err != nil {
		return err
	}
	// Save GPG primary key.
	if _, err = e.Insert(key); err != nil {
		return err
	}
	// Save GPG subs key.
	for _, subkey := range key.SubsKey {
		if err := addGPGSubKey(e, subkey); err != nil {
			return err
		}
	}
	return nil
}

// addGPGSubKey add subkeys to database
func addGPGSubKey(e db.Engine, key *GPGKey) (err error) {
	// Save GPG primary key.
	if _, err = e.Insert(key); err != nil {
		return err
	}
	// Save GPG subs key.
	for _, subkey := range key.SubsKey {
		if err := addGPGSubKey(e, subkey); err != nil {
			return err
		}
	}
	return nil
}

// AddGPGKey adds new public key to database.
func AddGPGKey(ownerID int64, content, token, signature string) ([]*GPGKey, error) {
	ekeys, err := checkArmoredGPGKeyString(content)
	if err != nil {
		return nil, err
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	keys := make([]*GPGKey, 0, len(ekeys))

	verified := false
	// Handle provided signature
	if signature != "" {
		signer, err := openpgp.CheckArmoredDetachedSignature(ekeys, strings.NewReader(token), strings.NewReader(signature))
		if err != nil {
			signer, err = openpgp.CheckArmoredDetachedSignature(ekeys, strings.NewReader(token+"\n"), strings.NewReader(signature))
		}
		if err != nil {
			signer, err = openpgp.CheckArmoredDetachedSignature(ekeys, strings.NewReader(token+"\r\n"), strings.NewReader(signature))
		}
		if err != nil {
			log.Error("Unable to validate token signature. Error: %v", err)
			return nil, ErrGPGInvalidTokenSignature{
				ID:      ekeys[0].PrimaryKey.KeyIdString(),
				Wrapped: err,
			}
		}
		ekeys = []*openpgp.Entity{signer}
		verified = true
	}

	if len(ekeys) > 1 {
		id2key := map[string]*openpgp.Entity{}
		newEKeys := make([]*openpgp.Entity, 0, len(ekeys))
		for _, ekey := range ekeys {
			id := ekey.PrimaryKey.KeyIdString()
			if original, has := id2key[id]; has {
				// Coalesce this with the other one
				for _, subkey := range ekey.Subkeys {
					if subkey.PublicKey == nil {
						continue
					}
					found := false

					for _, originalSubkey := range original.Subkeys {
						if originalSubkey.PublicKey == nil {
							continue
						}
						if originalSubkey.PublicKey.KeyId == subkey.PublicKey.KeyId {
							found = true
							break
						}
					}
					if !found {
						original.Subkeys = append(original.Subkeys, subkey)
					}
				}
				for name, identity := range ekey.Identities {
					if _, has := original.Identities[name]; has {
						continue
					}
					original.Identities[name] = identity
				}
				continue
			}
			id2key[id] = ekey
			newEKeys = append(newEKeys, ekey)
		}
		ekeys = newEKeys
	}

	for _, ekey := range ekeys {
		// Key ID cannot be duplicated.
		has, err := db.GetEngine(ctx).Where("key_id=?", ekey.PrimaryKey.KeyIdString()).
			Get(new(GPGKey))
		if err != nil {
			return nil, err
		} else if has {
			return nil, ErrGPGKeyIDAlreadyUsed{ekey.PrimaryKey.KeyIdString()}
		}

		// Get DB session

		key, err := parseGPGKey(ownerID, ekey, verified)
		if err != nil {
			return nil, err
		}

		if err = addGPGKey(db.GetEngine(ctx), key, content); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, committer.Commit()
}
