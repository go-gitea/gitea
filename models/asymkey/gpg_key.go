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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/packet"
	"xorm.io/xorm"
)

//   __________________  ________   ____  __.
//  /  _____/\______   \/  _____/  |    |/ _|____ ___.__.
// /   \  ___ |     ___/   \  ___  |      <_/ __ <   |  |
// \    \_\  \|    |   \    \_\  \ |    |  \  ___/\___  |
//	\______  /|____|    \______  / |____|__ \___  > ____|
//				 \/                  \/          \/   \/\/

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

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *GPGKey) AfterLoad(session *xorm.Session) {
	err := session.Where("primary_key_id=?", key.KeyID).Find(&key.SubsKey)
	if err != nil {
		log.Error("Find Sub GPGkeys[%s]: %v", key.KeyID, err)
	}
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

// ListGPGKeys returns a list of public keys belongs to given user.
func ListGPGKeys(ctx context.Context, uid int64, listOptions db.ListOptions) ([]*GPGKey, error) {
	sess := db.GetEngine(ctx).Table(&GPGKey{}).Where("owner_id=? AND primary_key_id=''", uid)
	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
	}

	keys := make([]*GPGKey, 0, 2)
	return keys, sess.Find(&keys)
}

// CountUserGPGKeys return number of gpg keys a user own
func CountUserGPGKeys(ctx context.Context, userID int64) (int64, error) {
	return db.GetEngine(ctx).Where("owner_id=? AND primary_key_id=''", userID).Count(&GPGKey{})
}

// GetGPGKeyByID returns public key by given ID.
func GetGPGKeyByID(ctx context.Context, keyID int64) (*GPGKey, error) {
	key := new(GPGKey)
	has, err := db.GetEngine(ctx).ID(keyID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGPGKeyNotExist{keyID}
	}
	return key, nil
}

// GetGPGKeysByKeyID returns public key by given ID.
func GetGPGKeysByKeyID(ctx context.Context, keyID string) ([]*GPGKey, error) {
	keys := make([]*GPGKey, 0, 1)
	return keys, db.GetEngine(ctx).Where("key_id=?", keyID).Find(&keys)
}

// GPGKeyToEntity retrieve the imported key and the traducted entity
func GPGKeyToEntity(ctx context.Context, k *GPGKey) (*openpgp.Entity, error) {
	impKey, err := GetGPGImportByKeyID(ctx, k.KeyID)
	if err != nil {
		return nil, err
	}
	keys, err := checkArmoredGPGKeyString(impKey.Content)
	if err != nil {
		return nil, err
	}
	return keys[0], err
}

// parseSubGPGKey parse a sub Key
func parseSubGPGKey(ownerID int64, primaryID string, pubkey *packet.PublicKey, expiry time.Time) (*GPGKey, error) {
	content, err := base64EncPubKey(pubkey)
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
		subs, err := parseSubGPGKey(ownerID, pubkey.KeyIdString(), k.PublicKey, expiry)
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
		if ident.Revocation != nil {
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

	content, err := base64EncPubKey(pubkey)
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
	key, err := GetGPGKeyByID(ctx, id)
	if err != nil {
		if IsErrGPGKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetPublicKeyByID: %w", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return ErrGPGKeyAccessDenied{doer.ID, key.ID}
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

func checkKeyEmails(ctx context.Context, email string, keys ...*GPGKey) (bool, string) {
	uid := int64(0)
	var userEmails []*user_model.EmailAddress
	var user *user_model.User
	for _, key := range keys {
		for _, e := range key.Emails {
			if e.IsActivated && (email == "" || strings.EqualFold(e.Email, email)) {
				return true, e.Email
			}
		}
		if key.Verified && key.OwnerID != 0 {
			if uid != key.OwnerID {
				userEmails, _ = user_model.GetEmailAddresses(ctx, key.OwnerID)
				uid = key.OwnerID
				user = &user_model.User{ID: uid}
				_, _ = user_model.GetUser(ctx, user)
			}
			for _, e := range userEmails {
				if e.IsActivated && (email == "" || strings.EqualFold(e.Email, email)) {
					return true, e.Email
				}
			}
			if user.KeepEmailPrivate && strings.EqualFold(email, user.GetEmail()) {
				return true, user.GetEmail()
			}
		}
	}
	return false, email
}
