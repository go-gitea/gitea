// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"golang.org/x/crypto/bcrypt"
)

type BCryptHasher struct {
	Cost int
}

func (h BCryptHasher) HashPassword(password, salt, config string) (string, error) {
	if config == "" {
		return h.HashPassword(password, salt, h.getConfigFromSetting())
	}
	if cost, err := strconv.Atoi(config); err == nil {
		var tempPasswd []byte
		tempPasswd, _ = bcrypt.GenerateFromPassword([]byte(password), cost)
		return fmt.Sprintf("$bcrypt$%s", tempPasswd), nil
	} else {
		return "", err
	}
}

func (h BCryptHasher) Validate(password, salt, hash string) bool {
	split := strings.SplitN(hash[1:], "$", 2)
	if bcrypt.CompareHashAndPassword([]byte(split[1]), []byte(password)) == nil {
		return true
	}
	return false
}

func (h BCryptHasher) getConfigFromHash(hash string) string {
	split := strings.SplitN(hash[1:], "$", 5)
	return split[3]
}

func (h BCryptHasher) getConfigFromSetting() string {
	if h.Cost == 0 {
		h.Cost = setting.BcryptCost
	}
	return strconv.Itoa(h.Cost)
}

func init() {
	DefaultHasher.Hashers["bcrypt"] = BCryptHasher{0}
}
