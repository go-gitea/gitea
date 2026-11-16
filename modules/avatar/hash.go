// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// HashAvatar generates a unique string for input ID and data.
// Because avatar is per-user, one user deletes their avatar should not affect another user who uses the same avatar.
func HashAvatar(uniqueID int64, data []byte) string {
	h := sha256.New()
	h.Write([]byte(strconv.FormatInt(uniqueID, 10)))
	h.Write([]byte{'-'}) // make sure the bytes in data won't conflict with ID
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
