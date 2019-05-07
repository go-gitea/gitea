// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secret

import (
	"crypto/rand"
	"encoding/base64"
)

// New creats a new secret
func New() (string, error) {
	return NewWithLength(32)
}

// NewWithLength creates a new secret for a given length
func NewWithLength(length int64) (string, error) {
	return randomString(length)
}

func randomBytes(len int64) ([]byte, error) {
	b := make([]byte, len)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func randomString(len int64) (string, error) {
	b, err := randomBytes(len)
	return base64.URLEncoding.EncodeToString(b), err
}
