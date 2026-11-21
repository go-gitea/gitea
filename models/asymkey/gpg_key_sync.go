// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"errors"
	"strings"

	"gitea.dev/models/db"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// SynchronizeGPGKeys reconciles the public keys managed by a user's login source.
// Invalid values are skipped and reported after synchronization.
func SynchronizeGPGKeys(ctx context.Context, ownerID, sourceID int64, values []string) error {
	if sourceID <= 0 {
		return errors.New("GPG key synchronization requires a login source")
	}

	var invalidKeys error
	entities := make(map[string]*openpgp.Entity)
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		keys, err := CheckArmoredGPGKeyString(value)
		if err != nil {
			invalidKeys = errors.Join(invalidKeys, err)
			continue
		}
		for _, entity := range keys {
			entities[entity.PrimaryKey.KeyIdString()] = entity
		}
	}

	err := db.WithTx(ctx, func(ctx context.Context) error {
		var managed []*GPGKey
		if err := db.GetEngine(ctx).Where("owner_id=? AND login_source_id=? AND primary_key_id=?", ownerID, sourceID, "").Find(&managed); err != nil {
			return err
		}
		for _, key := range managed {
			if _, present := entities[key.KeyID]; present {
				delete(entities, key.KeyID)
				continue
			}
			if err := deleteGPGKey(ctx, key); err != nil {
				return err
			}
		}

		for keyID, entity := range entities {
			exists, err := db.GetEngine(ctx).Where("key_id=?", keyID).Exist(new(GPGKey))
			if err != nil {
				return err
			}
			if exists {
				continue
			}

			key, err := parseGPGKey(ctx, ownerID, entity, false)
			if IsErrGPGNoEmailFound(err) {
				invalidKeys = errors.Join(invalidKeys, err)
				continue
			}
			if err != nil {
				return err
			}
			key.LoginSourceID = sourceID

			// Store each certificate separately so removing one also removes its exported material.
			var content strings.Builder
			writer, err := armor.Encode(&content, openpgp.PublicKeyType, nil)
			if err != nil {
				return err
			}
			if err := entity.Serialize(writer); err != nil {
				return err
			}
			if err := writer.Close(); err != nil {
				return err
			}
			if err := addGPGKey(ctx, key, content.String()); err != nil {
				return err
			}
		}
		return nil
	})
	return errors.Join(err, invalidKeys)
}
