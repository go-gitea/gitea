// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ledger

import (
	"bufio"
	"encoding/json"
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

// ReceiptSliceResult holds the result of a receipt slice query
type ReceiptSliceResult struct {
	Lines      []string // raw JSONL lines
	NextCursor int64    // byte offset for next page
}

// ReceiptSliceOptions configures receipt slicing
type ReceiptSliceOptions struct {
	Cursor    int64  // start reading from this byte offset
	SinceTS   int64  // only include receipts with ts_unix_ms >= this (0 = no filter)
	SinceRoot string // only include receipts after this root (exclusive, "" = no filter)
	Limit     int    // max receipts to return (0 = unlimited)
}

// ReadReceiptSlice reads receipts from JSONL with cursor-based pagination and filtering
func (l *Ledger) ReadReceiptSlice(opts ReceiptSliceOptions) (*ReceiptSliceResult, error) {
	logPath := filepath.Join(l.Dir, "receipts.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ReceiptSliceResult{Lines: []string{}, NextCursor: 0}, nil
		}
		return nil, err
	}
	defer f.Close()

	// Seek to cursor position
	if opts.Cursor > 0 {
		if _, err := f.Seek(opts.Cursor, io.SeekStart); err != nil {
			return nil, err
		}
	}

	result := &ReceiptSliceResult{Lines: make([]string, 0)}
	scanner := bufio.NewScanner(f)
	currentPos := opts.Cursor
	foundSinceRoot := opts.SinceRoot == "" // if no root filter, start immediately
	count := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineBytes := len(line) + 1 // +1 for newline

		// Parse minimal fields for filtering
		var r struct {
			TsUnixMs int64  `json:"ts_unix_ms"`
			Root     string `json:"root"`
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			// Skip malformed lines
			currentPos += int64(lineBytes)
			continue
		}

		// Apply since_root filter (exclusive - skip until we see the root, then start emitting after)
		if !foundSinceRoot {
			if r.Root == opts.SinceRoot {
				foundSinceRoot = true
			}
			currentPos += int64(lineBytes)
			continue
		}

		// Apply since_ts filter (inclusive)
		if opts.SinceTS > 0 && r.TsUnixMs < opts.SinceTS {
			currentPos += int64(lineBytes)
			continue
		}

		// Include this receipt
		result.Lines = append(result.Lines, line)
		count++
		currentPos += int64(lineBytes)

		// Check limit
		if opts.Limit > 0 && count >= opts.Limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result.NextCursor = currentPos
	return result, nil
}
