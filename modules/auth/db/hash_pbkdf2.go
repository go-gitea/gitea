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

// Pbkdf2Hasher is a Hash implementation for Pbkdf2
type Pbkdf2Hasher struct {
	Iterations int
	KeyLength  int
}

// HashPassword returns a PasswordHash, PassWordAlgo (and optionally an error)
func (h Pbkdf2Hasher) HashPassword(password, salt, config string) (string, string, error) {
	var tempPasswd []byte
	if config == "fallback" {
		config = "10000$50"
	}

	split := strings.Split(config, "$")
	if len(split) != 2 {
		split = strings.Split(h.getConfigFromSetting(), "$")
	}

	var iterations, parallelism int
	var err error

	if iterations, err = strconv.Atoi(split[0]); err != nil {
		return "", "", err
	}
	if parallelism, err = strconv.Atoi(split[1]); err != nil {
		return "", "", err
	}

	tempPasswd = pbkdf2.Key([]byte(password), []byte(salt), iterations, parallelism, sha256.New)
	return fmt.Sprintf("%x", tempPasswd),
		fmt.Sprintf("pbkdf2$%d$%d", iterations, parallelism),
		nil
}

// Validate validates a plain-text password
func (h Pbkdf2Hasher) Validate(password, hash, salt, config string) bool {
	tempHash, _, _ := h.HashPassword(password, salt, config)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(tempHash)) == 1
}

func (h Pbkdf2Hasher) getConfigFromAlgo(algo string) string {
	split := strings.SplitN(algo, "$", 2)
	if len(split) == 1 {
		split[1] = "fallback"
	}
	return split[1]
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
