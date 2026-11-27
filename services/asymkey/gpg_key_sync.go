// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/services/audit"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"xorm.io/builder"
)

// OpenPGP stores identities in a map. Write them in a stable order so an unchanged
// certificate always produces the same armor, including when it has multiple UIDs.
func armorGPGPublicKey(entity *openpgp.Entity) (string, error) {
	var content strings.Builder
	writer, err := armor.Encode(&content, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", err
	}
	packets := []interface{ Serialize(io.Writer) error }{entity.PrimaryKey}
	for _, sig := range entity.Revocations {
		packets = append(packets, sig)
	}
	for _, sig := range entity.Signatures {
		packets = append(packets, sig)
	}
	for _, name := range slices.Sorted(maps.Keys(entity.Identities)) {
		identity := entity.Identities[name]
		packets = append(packets, identity.UserId)
		for _, sig := range identity.Signatures {
			packets = append(packets, sig)
		}
	}
	for _, sub := range entity.Subkeys {
		packets = append(packets, sub.PublicKey)
		for _, sig := range sub.Revocations {
			packets = append(packets, sig)
		}
		packets = append(packets, sub.Sig)
	}
	for _, p := range packets {
		if err := p.Serialize(writer); err != nil {
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return content.String(), nil
}

// SynchronizeGPGKeys reconciles only the certificates managed by this login source.
func SynchronizeGPGKeys(ctx context.Context, user *user_model.User, sourceID int64, values []string) error {
	if user.ID <= 0 || sourceID <= 0 {
		return errors.New("GPG key synchronization requires a user and a login source")
	}

	var invalidKeys error
	entities := make(map[string]*openpgp.Entity)
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		keys, err := asymkey_model.CheckArmoredGPGKeyString(value)
		if err != nil {
			invalidKeys = errors.Join(invalidKeys, err)
			continue
		}
		for _, entity := range keys {
			entities[entity.PrimaryKey.KeyIdString()] = entity
		}
	}

	// A sign-in and scheduled sync can reconcile the same user at the same time.
	release, err := globallock.Lock(ctx, fmt.Sprintf("gpg_key_sync_%d", user.ID))
	if err != nil {
		return err
	}
	defer release()

	err = db.WithTx(ctx, func(ctx context.Context) error {
		managed, err := db.Find[asymkey_model.GPGKey](ctx, asymkey_model.FindGPGKeyOptions{OwnerID: user.ID, LoginSourceID: sourceID})
		if err != nil {
			return err
		}
		actor := user_model.NewAuthSourceUser()
		for _, key := range managed {
			if _, present := entities[key.KeyID]; present {
				continue
			}
			if err := asymkey_model.DeleteGPGKey(ctx, user, key.ID); err != nil {
				return err
			}
			audit.RecordAs(ctx, actor, audit_model.UserKeyGPGRemove, user, "gpg_key_id", key.KeyID)
		}

		for id, entity := range entities {
			existing, found, err := db.Get[asymkey_model.GPGKey](ctx, builder.Eq{"key_id": id})
			if err != nil {
				return err
			}
			if found && existing.OwnerID != user.ID {
				invalidKeys = errors.Join(invalidKeys, asymkey_model.ErrGPGKeyIDAlreadyUsed{KeyID: id})
				continue
			}
			if found && (existing.LoginSourceID != sourceID || existing.PrimaryKeyID != "") {
				continue
			}
			content, err := armorGPGPublicKey(entity)
			if err != nil {
				return err
			}
			if found {
				imported, err := asymkey_model.GetGPGImportByKeyID(ctx, id)
				if err != nil {
					return err
				}
				if imported.Content == content {
					continue
				}
			}
			err = asymkey_model.StoreGPGKeyFromSource(ctx, user.ID, sourceID, entity, content, existing)
			if asymkey_model.IsErrGPGNoEmailFound(err) || asymkey_model.IsErrGPGKeyIDAlreadyUsed(err) {
				invalidKeys = errors.Join(invalidKeys, err)
				continue
			}
			if err != nil {
				return err
			}
			if found {
				audit.RecordAs(ctx, actor, audit_model.UserKeyGPGRemove, user, "gpg_key_id", id)
			}
			audit.RecordAs(ctx, actor, audit_model.UserKeyGPGAdd, user, "gpg_key_id", id)
		}
		return nil
	})
	return errors.Join(err, invalidKeys)
}
