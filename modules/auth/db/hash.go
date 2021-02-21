// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

type DefaultHasherStruct struct {
	Hashers map[string]Hasher
}

type Hasher interface {
	Validate(password, salt, hash string) bool
	HashPassword(password, salt, config string) (string, error)
	getConfigFromSetting() string
	getConfigFromHash(hash string) string
}

func (d DefaultHasherStruct) Validate(password, salt, hash string) bool {
	var typ, tail string
	var hasher Hasher
	var ok bool
	split := strings.SplitN(hash[1:], "$", 2)
	typ, tail = split[0], split[1]

	if len(tail) == 0 || len(typ) == 0 {
		return false
	}

	if hasher, ok = d.Hashers[typ]; ok {
		return hasher.Validate(password, salt, hash)
	}
	return false
}

func (d DefaultHasherStruct) HashPassword(password, salt, config string) (string, error) {
	return d.Hashers[setting.PasswordHashAlgo].HashPassword(password, salt, config)
}

func (d DefaultHasherStruct) PasswordNeedUpdate(hash string) bool {
	var typ, tail string
	var hasher Hasher
	var ok bool
	split := strings.SplitN(hash[1:], "$", 2)
	typ, tail = split[0], split[1]

	if len(tail) == 0 || len(typ) == 0 || typ != setting.PasswordHashAlgo {
		return true
	}

	if hasher, ok = d.Hashers[typ]; ok {
		return hasher.getConfigFromHash(hash) != hasher.getConfigFromSetting()
	}
	return true
}

var DefaultHasher DefaultHasherStruct

func init() {
	DefaultHasher = DefaultHasherStruct{}
	DefaultHasher.Hashers = make(map[string]Hasher)
}
