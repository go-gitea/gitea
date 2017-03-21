// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/go-xorm/xorm"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

// GPGKey represents a GPG key.
type GPGKey struct {
	ID                int64     `xorm:"pk autoincr"`
	OwnerID           int64     `xorm:"INDEX NOT NULL"`
	KeyID             string    `xorm:"INDEX CHAR(16) NOT NULL"`
	PrimaryKeyID      string    `xorm:"CHAR(16)"`
	Content           string    `xorm:"TEXT NOT NULL"`
	Created           time.Time `xorm:"-"`
	CreatedUnix       int64
	Expired           time.Time `xorm:"-"`
	ExpiredUnix       int64
	Added             time.Time `xorm:"-"`
	AddedUnix         int64
	SubsKey           []*GPGKey `xorm:"-"`
	Emails            []*EmailAddress
	CanSign           bool
	CanEncryptComms   bool
	CanEncryptStorage bool
	CanCertify        bool
}

// BeforeInsert will be invoked by XORM before inserting a record
func (key *GPGKey) BeforeInsert() {
	key.AddedUnix = time.Now().Unix()
	key.ExpiredUnix = key.Expired.Unix()
	key.CreatedUnix = key.Created.Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (key *GPGKey) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "key_id":
		x.Where("primary_key_id=?", key.KeyID).Find(&key.SubsKey)
	case "added_unix":
		key.Added = time.Unix(key.AddedUnix, 0).Local()
	case "expired_unix":
		key.Expired = time.Unix(key.ExpiredUnix, 0).Local()
	case "created_unix":
		key.Created = time.Unix(key.CreatedUnix, 0).Local()
	}
}

// ListGPGKeys returns a list of public keys belongs to given user.
func ListGPGKeys(uid int64) ([]*GPGKey, error) {
	keys := make([]*GPGKey, 0, 5)
	return keys, x.Where("owner_id=? AND primary_key_id=''", uid).Find(&keys)
}

// GetGPGKeyByID returns public key by given ID.
func GetGPGKeyByID(keyID int64) (*GPGKey, error) {
	key := new(GPGKey)
	has, err := x.Id(keyID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGPGKeyNotExist{keyID}
	}
	return key, nil
}

// checkArmoredGPGKeyString checks if the given key string is a valid GPG armored key.
// The function returns the actual public key on success
func checkArmoredGPGKeyString(content string) (*openpgp.Entity, error) {
	list, err := openpgp.ReadArmoredKeyRing(strings.NewReader(content))
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

//addGPGKey add key and subkeys to database
func addGPGKey(e Engine, key *GPGKey) (err error) {
	// Save GPG primary key.
	if _, err = e.Insert(key); err != nil {
		return err
	}
	// Save GPG subs key.
	for _, subkey := range key.SubsKey {
		if err := addGPGKey(e, subkey); err != nil {
			return err
		}
	}
	return nil
}

// AddGPGKey adds new public key to database.
func AddGPGKey(ownerID int64, content string) (*GPGKey, error) {
	ekey, err := checkArmoredGPGKeyString(content)
	if err != nil {
		return nil, err
	}

	// Key ID cannot be duplicated.
	has, err := x.Where("key_id=?", ekey.PrimaryKey.KeyIdString()).
		Get(new(GPGKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, ErrGPGKeyIDAlreadyUsed{ekey.PrimaryKey.KeyIdString()}
	}

	//Get DB session
	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	key, err := parseGPGKey(ownerID, ekey)
	if err != nil {
		return nil, err
	}

	if err = addGPGKey(sess, key); err != nil {
		return nil, err
	}

	return key, sess.Commit()
}

//base64EncPubKey encode public kay content to base 64
func base64EncPubKey(pubkey *packet.PublicKey) (string, error) {
	var w bytes.Buffer
	err := pubkey.Serialize(&w)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(w.Bytes()), nil
}

//parseSubGPGKey parse a sub Key
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
		Created:           pubkey.CreationTime,
		Expired:           expiry,
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

//parseGPGKey parse a PrimaryKey entity (primary key + subs keys + self-signature)
func parseGPGKey(ownerID int64, e *openpgp.Entity) (*GPGKey, error) {
	pubkey := e.PrimaryKey

	//Extract self-sign for expire date based on : https://github.com/golang/crypto/blob/master/openpgp/keys.go#L165
	var selfSig *packet.Signature
	for _, ident := range e.Identities {
		if selfSig == nil {
			selfSig = ident.SelfSignature
		} else if ident.SelfSignature.IsPrimaryId != nil && *ident.SelfSignature.IsPrimaryId {
			selfSig = ident.SelfSignature
			break
		}
	}
	expiry := time.Time{}
	if selfSig.KeyLifetimeSecs != nil {
		expiry = selfSig.CreationTime.Add(time.Duration(*selfSig.KeyLifetimeSecs) * time.Second)
	}

	//Parse Subkeys
	subkeys := make([]*GPGKey, len(e.Subkeys))
	for i, k := range e.Subkeys {
		subs, err := parseSubGPGKey(ownerID, pubkey.KeyIdString(), k.PublicKey, expiry)
		if err != nil {
			return nil, err
		}
		subkeys[i] = subs
	}

	//Check emails
	userEmails, err := GetEmailAddresses(ownerID)
	if err != nil {
		return nil, err
	}
	emails := make([]*EmailAddress, len(e.Identities))
	n := 0
	for _, ident := range e.Identities {

		for _, e := range userEmails {
			if e.Email == ident.UserId.Email && e.IsActivated {
				emails[n] = e
				break
			}
		}
		if emails[n] == nil {
			return nil, fmt.Errorf("Failed to found email or is not confirmed : %s", ident.UserId.Email)
		}
		n++
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
		Created:           pubkey.CreationTime,
		Expired:           expiry,
		Emails:            emails,
		SubsKey:           subkeys,
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

// deleteGPGKey does the actual key deletion
func deleteGPGKey(e *xorm.Session, keyID string) (int64, error) {
	if keyID == "" {
		return 0, fmt.Errorf("empty KeyId forbidden") //Should never happen but just to be sure
	}
	return e.Where("key_id=?", keyID).Or("primary_key_id=?", keyID).Delete(new(GPGKey))
}

// DeleteGPGKey deletes GPG key information in database.
func DeleteGPGKey(doer *User, id int64) (err error) {
	key, err := GetGPGKeyByID(id)
	if err != nil {
		if IsErrGPGKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetPublicKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return ErrGPGKeyAccessDenied{doer.ID, key.ID}
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = deleteGPGKey(sess, key.KeyID); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	return nil
}
