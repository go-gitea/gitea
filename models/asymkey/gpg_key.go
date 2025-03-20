// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"xorm.io/builder"
)

// GPGKey represents a GPG key.
type GPGKey struct {
	ID                int64              `xorm:"pk autoincr"`
	OwnerID           int64              `xorm:"INDEX NOT NULL"`
	KeyID             string             `xorm:"INDEX CHAR(16) NOT NULL"`
	PrimaryKeyID      string             `xorm:"CHAR(16)"`
	Content           string             `xorm:"MEDIUMTEXT NOT NULL"`
	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	ExpiredUnix       timeutil.TimeStamp
	AddedUnix         timeutil.TimeStamp
	SubsKey           []*GPGKey `xorm:"-"`
	Emails            []*user_model.EmailAddress
	Verified          bool `xorm:"NOT NULL DEFAULT false"`
	CanSign           bool
	CanEncryptComms   bool
	CanEncryptStorage bool
	CanCertify        bool
}

func init() {
	db.RegisterModel(new(GPGKey))
}

// BeforeInsert will be invoked by XORM before inserting a record
func (key *GPGKey) BeforeInsert() {
	key.AddedUnix = timeutil.TimeStampNow()
}

func (key *GPGKey) LoadSubKeys(ctx context.Context) error {
	if err := db.GetEngine(ctx).Where("primary_key_id=?", key.KeyID).Find(&key.SubsKey); err != nil {
		return fmt.Errorf("find Sub GPGkeys[%s]: %v", key.KeyID, err)
	}
	return nil
}

// PaddedKeyID show KeyID padded to 16 characters
func (key *GPGKey) PaddedKeyID() string {
	return PaddedKeyID(key.KeyID)
}

// PaddedKeyID show KeyID padded to 16 characters
func PaddedKeyID(keyID string) string {
	if len(keyID) > 15 {
		return keyID
	}
	zeros := "0000000000000000"
	return zeros[0:16-len(keyID)] + keyID
}

type FindGPGKeyOptions struct {
	db.ListOptions
	OwnerID        int64
	KeyID          string
	IncludeSubKeys bool
}

func (opts FindGPGKeyOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if !opts.IncludeSubKeys {
		cond = cond.And(builder.Eq{"primary_key_id": ""})
	}

	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.KeyID != "" {
		cond = cond.And(builder.Eq{"key_id": opts.KeyID})
	}
	return cond
}

func GetGPGKeyForUserByID(ctx context.Context, ownerID, keyID int64) (*GPGKey, error) {
	key := new(GPGKey)
	has, err := db.GetEngine(ctx).Where("id=? AND owner_id=?", keyID, ownerID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGPGKeyNotExist{keyID}
	}
	return key, nil
}

// GPGKeyToEntity retrieve the imported key and the traducted entity
func GPGKeyToEntity(ctx context.Context, k *GPGKey) (*openpgp.Entity, error) {
	impKey, err := GetGPGImportByKeyID(ctx, k.KeyID)
	if err != nil {
		return nil, err
	}
	keys, err := CheckArmoredGPGKeyString(impKey.Content)
	if err != nil {
		return nil, err
	}
	return keys[0], err
}

// parseSubGPGKey parse a sub Key
func parseSubGPGKey(ownerID int64, primaryID string, pubkey *packet.PublicKey, expiry time.Time) (*GPGKey, error) {
	content, err := Base64EncPubKey(pubkey)
	if err != nil {
		return nil, err
	}
	return &GPGKey{
		OwnerID:           ownerID,
		KeyID:             pubkey.KeyIdString(),
		PrimaryKeyID:      primaryID,
		Content:           content,
		CreatedUnix:       timeutil.TimeStamp(pubkey.CreationTime.Unix()),
		ExpiredUnix:       timeutil.TimeStamp(expiry.Unix()),
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

// parseGPGKey parse a PrimaryKey entity (primary key + subs keys + self-signature)
func parseGPGKey(ctx context.Context, ownerID int64, e *openpgp.Entity, verified bool) (*GPGKey, error) {
	pubkey := e.PrimaryKey
	expiry := getExpiryTime(e)

	// Parse Subkeys
	subkeys := make([]*GPGKey, len(e.Subkeys))
	for i, k := range e.Subkeys {
		subkeyExpiry := expiry
		if k.Sig.KeyLifetimeSecs != nil {
			subkeyExpiry = k.PublicKey.CreationTime.Add(time.Duration(*k.Sig.KeyLifetimeSecs) * time.Second)
		}
		subs, err := parseSubGPGKey(ownerID, pubkey.KeyIdString(), k.PublicKey, subkeyExpiry)
		if err != nil {
			return nil, ErrGPGKeyParsing{ParseError: err}
		}
		subkeys[i] = subs
	}

	// Check emails
	userEmails, err := user_model.GetEmailAddresses(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	emails := make([]*user_model.EmailAddress, 0, len(e.Identities))
	for _, ident := range e.Identities {
		if ident.Revoked(time.Now()) {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(ident.UserId.Email))
		for _, e := range userEmails {
			if e.IsActivated && e.LowerEmail == email {
				emails = append(emails, e)
				break
			}
		}
	}

	if !verified {
		// In the case no email as been found
		if len(emails) == 0 {
			failedEmails := make([]string, 0, len(e.Identities))
			for _, ident := range e.Identities {
				failedEmails = append(failedEmails, ident.UserId.Email)
			}
			return nil, ErrGPGNoEmailFound{failedEmails, e.PrimaryKey.KeyIdString()}
		}
	}

	content, err := Base64EncPubKey(pubkey)
	if err != nil {
		return nil, err
	}
	return &GPGKey{
		OwnerID:           ownerID,
		KeyID:             pubkey.KeyIdString(),
		PrimaryKeyID:      "",
		Content:           content,
		CreatedUnix:       timeutil.TimeStamp(pubkey.CreationTime.Unix()),
		ExpiredUnix:       timeutil.TimeStamp(expiry.Unix()),
		Emails:            emails,
		SubsKey:           subkeys,
		Verified:          verified,
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

// deleteGPGKey does the actual key deletion
func deleteGPGKey(ctx context.Context, keyID string) (int64, error) {
	if keyID == "" {
		return 0, fmt.Errorf("empty KeyId forbidden") // Should never happen but just to be sure
	}
	// Delete imported key
	n, err := db.GetEngine(ctx).Where("key_id=?", keyID).Delete(new(GPGKeyImport))
	if err != nil {
		return n, err
	}
	return db.GetEngine(ctx).Where("key_id=?", keyID).Or("primary_key_id=?", keyID).Delete(new(GPGKey))
}

// DeleteGPGKey deletes GPG key information in database.
func DeleteGPGKey(ctx context.Context, doer *user_model.User, id int64) (err error) {
	key, err := GetGPGKeyForUserByID(ctx, doer.ID, id)
	if err != nil {
		if IsErrGPGKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetPublicKeyByID: %w", err)
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if _, err = deleteGPGKey(ctx, key.KeyID); err != nil {
		return err
	}

	return committer.Commit()
}
