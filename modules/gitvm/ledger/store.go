// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ledger

import (
	"io"
	"os"
	"path/filepath"
)

// writeAtomic atomically writes data to a file using a temp file + rename
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // cleanup on error

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// ReadReceipts reads all receipts from the JSONL file
func (l *Ledger) ReadReceipts() ([]*Receipt, error) {
	logPath := filepath.Join(l.Dir, "receipts.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var receipts []*Receipt
	decoder := newJSONDecoder(f)
	for {
		var r Receipt
		if err := decoder.Decode(&r); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		receipts = append(receipts, &r)
	}
	return receipts, nil
}
