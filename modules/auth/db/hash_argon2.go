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

	"golang.org/x/crypto/argon2"
)

type Argon2Hasher struct {
	Iterations  uint32
	Memory      uint32
	Parallelism uint8
	KeyLength   uint32
}

func (h Argon2Hasher) HashPassword(password, salt, config string) (string, error) {
	var tempPasswd []byte
	split := strings.Split(config, "$")
	if len(split) != 4 {
		return h.HashPassword(password, salt, h.getConfigFromSetting())
	}

	var iterations, memory, keyLength uint32
	var parallelism uint8
	var tmp int

	var err error

	if tmp, err = strconv.Atoi(split[0]); err != nil {
		return "", err
	} else {
		iterations = uint32(tmp)
	}
	if tmp, err = strconv.Atoi(split[1]); err != nil {
		return "", err
	} else {
		memory = uint32(tmp)
	}
	if tmp, err = strconv.Atoi(split[2]); err != nil {
		return "", err
	} else {
		parallelism = uint8(tmp)
	}
	if tmp, err = strconv.Atoi(split[3]); err != nil {
		return "", err
	} else {
		keyLength = uint32(tmp)
	}

	tempPasswd = argon2.IDKey([]byte(password), []byte(salt), iterations, memory, parallelism, keyLength)
	return fmt.Sprintf("$argon2$%d$%d$%d$%d$%x", iterations, memory, parallelism, keyLength, tempPasswd), nil
}

func (h Argon2Hasher) Validate(password, salt, hash string) bool {
	tempHash, _ := h.HashPassword(password, salt, h.getConfigFromHash(hash))
	if subtle.ConstantTimeCompare([]byte(hash), []byte(tempHash)) == 1 {
		return true
	}
	return false
}

func (h Argon2Hasher) getConfigFromHash(hash string) string {
	configEnd := strings.LastIndex(hash, "$")
	return hash[8:configEnd]
}

func (h Argon2Hasher) getConfigFromSetting() string {
	if h.Iterations == 0 {
		h.Iterations = setting.Argon2Iterations
		h.Memory = setting.Argon2Memory
		h.Parallelism = setting.Argon2Parallelism
		h.KeyLength = setting.Argon2KeyLength
	}
	return fmt.Sprintf("%d$%d$%d$%d", h.Iterations, h.Memory, h.Parallelism, h.KeyLength)
}

func init() {
	DefaultHasher.Hashers["argon2"] = Argon2Hasher{0, 0, 0, 0}
}
