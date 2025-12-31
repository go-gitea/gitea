// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ledger

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zeebo/blake3"
	"golang.org/x/sys/unix"
)

// Ledger manages the append-only receipt log
type Ledger struct {
	Dir string
}

// New creates a new ledger instance
func New(dir string) *Ledger {
	return &Ledger{Dir: dir}
}

// Emit appends a new receipt to the ledger and updates the rolling root
func (l *Ledger) Emit(receipt *Receipt) error {
	if err := os.MkdirAll(l.Dir, 0o700); err != nil {
		return err
	}

	// lock file (process-safe)
	lockPath := filepath.Join(l.Dir, ".lock")
	lockFd, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lockFd.Close()
	if err := unix.Flock(int(lockFd.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lockFd.Fd()), unix.LOCK_UN)

	// load prev root
	prevRoot := l.readRoot()
	receipt.PrevRoot = prevRoot
	if receipt.TsUnixMs == 0 {
		receipt.TsUnixMs = time.Now().UnixMilli()
	}
	receipt.Version = 1

	// canonical bytes: marshal a copy WITHOUT ReceiptHash/Root set
	tmp := *receipt
	tmp.ReceiptHash = ""
	tmp.Root = ""
	b, err := json.Marshal(&tmp)
	if err != nil {
		return err
	}

	rh := blake3.Sum256(b)
	receipt.ReceiptHash = "b3:" + hex32(rh[:])

	// new root = BLAKE3(prevRoot || "|" || receiptHash)
	rootBytes := blake3.Sum256([]byte(prevRoot + "|" + receipt.ReceiptHash))
	receipt.Root = "b3:" + hex32(rootBytes[:])

	// append JSONL
	logPath := filepath.Join(l.Dir, "receipts.jsonl")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	out, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if _, err := w.Write(out); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}

	// write ROOT.txt (atomic)
	return writeAtomic(filepath.Join(l.Dir, "ROOT.txt"), []byte(receipt.Root+"\n"), 0o600)
}

// GetRoot reads the current root from ROOT.txt
func (l *Ledger) GetRoot() (string, error) {
	root := l.readRoot()
	if root == "" {
		return "", nil
	}
	return root, nil
}

// newJSONDecoder creates a JSON decoder that reads line by line
func newJSONDecoder(r io.Reader) *json.Decoder {
	return json.NewDecoder(r)
}
