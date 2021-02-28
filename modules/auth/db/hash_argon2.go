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

// Argon2Hasher is a Hash implementation for Argon2
type Argon2Hasher struct {
	Iterations  uint32
	Memory      uint32
	Parallelism uint8
	KeyLength   uint32
}

// HashPassword returns a PasswordHash, PassWordAlgo (and optionally an error)
func (h Argon2Hasher) HashPassword(password, salt, config string) (string, string, error) {
	var tempPasswd []byte
	if config == "fallback" {
		config = "2$65536$8$50"
	}

	split := strings.Split(config, "$")
	if len(split) != 4 {
		fmt.Printf("Take from Config: %v", h.getConfigFromSetting())
		split = strings.Split(h.getConfigFromSetting(), "$")
	}

	var iterations, memory, keyLength uint32
	var parallelism uint8
	var tmp int

	var err error

	if tmp, err = strconv.Atoi(split[0]); err != nil {
		return "", "", err
	}
	iterations = uint32(tmp)

	if tmp, err = strconv.Atoi(split[1]); err != nil {
		return "", "", err
	}
	memory = uint32(tmp)
	if tmp, err = strconv.Atoi(split[2]); err != nil {
		return "", "", err
	}
	parallelism = uint8(tmp)
	if tmp, err = strconv.Atoi(split[3]); err != nil {
		return "", "", err
	}
	keyLength = uint32(tmp)

	tempPasswd = argon2.IDKey([]byte(password), []byte(salt), iterations, memory, parallelism, keyLength)
	return fmt.Sprintf("%x", tempPasswd),
		fmt.Sprintf("argon2$%d$%d$%d$%d", iterations, memory, parallelism, keyLength),
		nil
}

// Validate validates a plain-text password
func (h Argon2Hasher) Validate(password, hash, salt, config string) bool {
	tempHash, _, _ := h.HashPassword(password, salt, config)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(tempHash)) == 1
}

func (h Argon2Hasher) getConfigFromAlgo(algo string) string {
	split := strings.SplitN(algo, "$", 2)
	if len(split) == 1 {
		split[1] = "fallback"
	}
	return split[1]
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
