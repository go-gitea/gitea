// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"crypto/subtle"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"golang.org/x/crypto/scrypt"
)

type SCryptHasher struct {
	N         int
	R         int
	P         int
	KeyLength int
}

func (h SCryptHasher) HashPassword(password, salt, config string) (string, error) {
	var tempPasswd []byte
	split := strings.Split(config, "$")
	if len(split) != 4 {
		return h.HashPassword(password, salt, h.getConfigFromSetting())
	}

	var n, r, p, keyLength int
	var err error

	if n, err = strconv.Atoi(split[0]); err != nil {
		return "", err
	}
	if r, err = strconv.Atoi(split[1]); err != nil {
		return "", err
	}
	if p, err = strconv.Atoi(split[2]); err != nil {
		return "", err
	}
	if keyLength, err = strconv.Atoi(split[3]); err != nil {
		return "", err
	}

	tempPasswd, _ = scrypt.Key([]byte(password), []byte(salt), n, r, p, keyLength)
	return fmt.Sprintf("$scrypt$%d$%d$%d$%d$%x", n, r, p, keyLength, tempPasswd), nil
}

func (h SCryptHasher) Validate(password, salt, hash string) bool {
	tempHash, _ := h.HashPassword(password, salt, h.getConfigFromHash(hash))
	if subtle.ConstantTimeCompare([]byte(hash), []byte(tempHash)) == 1 {
		return true
	}
	return false
}

func (h SCryptHasher) getConfigFromHash(hash string) string {
	configEnd := strings.LastIndex(hash, "$")
	return hash[8:configEnd]
}

func (h SCryptHasher) getConfigFromSetting() string {
	if h.KeyLength == 0 {
		h.N = setting.ScryptN
		h.R = setting.ScryptR
		h.P = setting.ScryptP
		h.KeyLength = setting.ScryptKeyLength
	}
	return fmt.Sprintf("%d$%d$%d$%d", h.N, h.R, h.P, h.KeyLength)
}

func init() {
	DefaultHasher.Hashers["scrypt"] = SCryptHasher{0, 0, 0, 0}
}
