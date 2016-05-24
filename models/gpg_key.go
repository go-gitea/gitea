// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"github.com/gogits/gogs/modules/log"
)

//TODO maybe refector with ssh-key ?
//TODO database behind

// PublicGPGKey represents a GPG key.
type PublicGPGKey struct {
	ID          int64  `xorm:"pk autoincr"`
	OwnerID     int64  `xorm:"INDEX NOT NULL"`
	Name        string `xorm:"NOT NULL"`
	Fingerprint string `xorm:"NOT NULL"`
	Content     string `xorm:"TEXT NOT NULL"`

	Created time.Time `xorm:"-"`
}

// ListPublicGPGKeys returns a list of public keys belongs to given user.
func ListPublicGPGKeys(uid int64) ([]*PublicGPGKey, error) {
	keys := make([]*PublicGPGKey, 0, 5)
	return keys, x.Where("owner_id=?", uid).Find(&keys)
}

// GetPublicGPGKeyByID returns public key by given ID.
func GetPublicGPGKeyByID(keyID int64) (*PublicGPGKey, error) {
	key := new(PublicGPGKey)
	has, err := x.Id(keyID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{keyID}
	}
	return key, nil
}

// CheckPublicGPGKeyString checks if the given public key string is a valid GPG key.
// The function returns the actual public key line on success.
func CheckPublicGPGKeyString(content string) (_ string, err error) {
	//TODO Implement
	return "", nil
}

// AddPublicGPGKey adds new public key to database.
func AddPublicGPGKey(ownerID int64, name, content string) (*PublicKey, error) {
	log.Trace(content)
	//TODO Implement
	return nil, nil
}

// DeletePublicGPGKey deletes GPG key information in database.
func DeletePublicGPGKey(doer *User, id int64) (err error) {
	//TODO Implement
	return nil
}
