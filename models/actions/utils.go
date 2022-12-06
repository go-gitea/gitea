// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

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

type LogIndexes []int64

func (indexes *LogIndexes) FromDB(b []byte) error {
	reader := bytes.NewReader(b)
	for {
		v, err := binary.ReadVarint(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("binary ReadVarint: %w", err)
		}
		*indexes = append(*indexes, v)
	}
}

func (indexes *LogIndexes) ToDB() ([]byte, error) {
	buf, i := make([]byte, binary.MaxVarintLen64*len(*indexes)), 0
	for _, v := range *indexes {
		n := binary.PutVarint(buf[i:], v)
		i += n
	}
	return buf[:i], nil
}
