// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// HashAvatar will generate a unique string, which ensures that when there's a
// different unique ID while the data is the same, it will generate a different
// output. It will generate the output according to:
// HEX(HASH(uniqueID || - || data))
// The hash being used is SHA256.
// The sole purpose of the unique ID is to generate a distinct hash Such that
// two unique IDs with the same data will have a different hash output.
// The "-" byte is important to ensure that data cannot be modified such that
// the first byte is a number, which could lead to a "collision" with the hash
// of another unique ID.
func HashAvatar(uniqueID int64, data []byte) string {
	h := sha256.New()
	h.Write([]byte(strconv.FormatInt(uniqueID, 10)))
	h.Write([]byte{'-'})
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
