// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"gitea.dev/modules/setting"
)

type tagType string

// BuildSignature builds a hmac signature for the input values.
// "tag" is an internal pre-defined static string to distinguish the signatures for different purpose.
func BuildSignature(tag tagType, vals ...string) []byte {
	m := hmac.New(sha256.New, setting.GetGeneralTokenSigningSecret())
	_, _ = io.WriteString(m, string(tag))
	var buf8 [8]byte
	for _, v := range vals {
		binary.LittleEndian.PutUint64(buf8[:], uint64(len(v)))
		_, _ = m.Write(buf8[:])
		_, _ = io.WriteString(m, v)
	}
	return m.Sum(nil)
}
