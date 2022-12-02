// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bots

import (
	"encoding/hex"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/util"
)

func generateSaltedToken() (string, string, string, string, error) {
	salt, err := util.CryptoRandomString(10)
	if err != nil {
		return "", "", "", "", err
	}
	buf, err := util.CryptoRandomBytes(20)
	if err != nil {
		return "", "", "", "", err
	}
	token := hex.EncodeToString(buf)
	hash := auth_model.HashToken(token, salt)
	return token, salt, hash, token[:8], nil
}
