// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

// DefaultHasherStruct stores the available hashing instances
type DefaultHasherStruct struct {
	Hashers map[string]Hasher
}

// Hasher is the interface for a single hash implementation
type Hasher interface {
	Validate(password, hash, salt, config string) bool
	HashPassword(password, salt, config string) (string, string, error)
	getConfigFromSetting() string
	getConfigFromAlgo(algo string) string
}

// HashPassword returns a PasswordHash, PassWordAlgo (and optionally an error)
func (d DefaultHasherStruct) HashPassword(password, salt, config string) (string, string, error) {
	if setting.PasswordHashAlgo == "" {
		setting.PasswordHashAlgo = "pbkdf2"
	}
	return d.Hashers[setting.PasswordHashAlgo].HashPassword(password, salt, config)
}

// Validate validates a plain-text password
func (d DefaultHasherStruct) Validate(password, hash, salt, algo string) bool {
	var typ, config string
	var hasher Hasher
	var ok bool
	split := strings.SplitN(algo, "$", 2)
	if len(split) == 1 {
		typ = split[0]
		config = "fallback"
	} else {
		typ, config = split[0], split[1]
	}

	if len(config) == 0 || len(typ) == 0 {
		return false
	}

	if hasher, ok = d.Hashers[typ]; ok {
		return hasher.Validate(password, hash, salt, config)
	}
	return false
}

// PasswordNeedUpdate determines if a password needs an update
func (d DefaultHasherStruct) PasswordNeedUpdate(algo string) bool {
	var typ, tail string
	var hasher Hasher
	var ok bool
	split := strings.SplitN(algo, "$", 2)
	if len(split) == 1 {
		return true
	}
	typ, tail = split[0], split[1]

	if len(tail) == 0 || len(typ) == 0 || typ != setting.PasswordHashAlgo {
		return true
	}

	if hasher, ok = d.Hashers[typ]; ok {
		return hasher.getConfigFromAlgo(algo) != hasher.getConfigFromSetting()
	}
	return true
}

// DefaultHasher is the instance of the HashSet
var DefaultHasher DefaultHasherStruct

func init() {
	DefaultHasher = DefaultHasherStruct{}
	DefaultHasher.Hashers = make(map[string]Hasher)
}
