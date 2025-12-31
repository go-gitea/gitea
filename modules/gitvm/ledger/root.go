// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ledger

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// readRoot reads the current root from ROOT.txt, returns empty string if not exists
func (l *Ledger) readRoot() string {
	rootPath := filepath.Join(l.Dir, "ROOT.txt")
	data, err := os.ReadFile(rootPath)
	if err != nil {
		return "" // first receipt, no previous root
	}
	return strings.TrimSpace(string(data))
}

// hex32 converts a 32-byte hash to hex string
func hex32(b []byte) string {
	return hex.EncodeToString(b)
}
