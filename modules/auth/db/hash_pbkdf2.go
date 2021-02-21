// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"golang.org/x/crypto/pbkdf2"
)

type Pbkdf2Hasher struct {
	Iterations int
	KeyLength  int
}

func (h Pbkdf2Hasher) HashPassword(password, salt, config string) (string, error) {
	var tempPasswd []byte
	split := strings.Split(config, "$")
	if len(split) != 2 {
		return h.HashPassword(password, salt, h.getConfigFromSetting())
	}

	var iterations, parallelism int
	var err error

	if iterations, err = strconv.Atoi(split[0]); err != nil {
		return "", err
	}
	if parallelism, err = strconv.Atoi(split[1]); err != nil {
		return "", err
	}

	tempPasswd = pbkdf2.Key([]byte(password), []byte(salt), iterations, parallelism, sha256.New)
	return fmt.Sprintf("$pbkdf2$%d$%d$%x", iterations, parallelism, tempPasswd), nil
}

func (h Pbkdf2Hasher) Validate(password, salt, hash string) bool {
	tempHash, _ := h.HashPassword(password, salt, h.getConfigFromHash(hash))
	if subtle.ConstantTimeCompare([]byte(hash), []byte(tempHash)) == 1 {
		return true
	}
	return false
}

func (h Pbkdf2Hasher) getConfigFromHash(hash string) string {
	configEnd := strings.LastIndex(hash, "$")
	return hash[8:configEnd]
}

func (h Pbkdf2Hasher) getConfigFromSetting() string {
	if h.KeyLength == 0 {
		h.Iterations = setting.Pbkdf2Iterations
		h.KeyLength = setting.Pbkdf2KeyLength
	}
	return fmt.Sprintf("%d$%d", h.Iterations, h.KeyLength)
}

func init() {
	DefaultHasher.Hashers["pbkdf2"] = Pbkdf2Hasher{0, 0}
}
