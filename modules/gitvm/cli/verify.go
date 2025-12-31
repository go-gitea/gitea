// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/gitvm/ledger"
	"github.com/zeebo/blake3"
)

// Verify verifies the integrity of the GitVM ledger
func Verify(dataDir string) error {
	ledgerDir := filepath.Join(dataDir, "gitvm")
	if dataDir == "" {
		ledgerDir = "./data/gitvm" // default
	}

	fmt.Printf("Verifying GitVM ledger at: %s\n", ledgerDir)

	// Read all receipts
	l := ledger.New(ledgerDir)
	receipts, err := l.ReadReceipts()
	if err != nil {
		return fmt.Errorf("failed to read receipts: %w", err)
	}

	if len(receipts) == 0 {
		fmt.Println("No receipts found. Ledger is empty.")
		return nil
	}

	fmt.Printf("Found %d receipts. Verifying...\n", len(receipts))

	// Recompute all hashes and roots
	prevRoot := ""
	for i, receipt := range receipts {
		// Check prev root
		if receipt.PrevRoot != prevRoot {
			return fmt.Errorf("receipt %d: prev_root mismatch (expected %s, got %s)", i, prevRoot, receipt.PrevRoot)
		}

		// Recompute receipt hash
		tmp := *receipt
		tmp.ReceiptHash = ""
		tmp.Root = ""
		b, err := json.Marshal(&tmp)
		if err != nil {
			return fmt.Errorf("receipt %d: failed to marshal: %w", i, err)
		}

		rh := blake3.Sum256(b)
		expectedHash := "b3:" + hex32(rh[:])
		if receipt.ReceiptHash != expectedHash {
			return fmt.Errorf("receipt %d: receipt_hash mismatch (expected %s, got %s)", i, expectedHash, receipt.ReceiptHash)
		}

		// Recompute root
		rootBytes := blake3.Sum256([]byte(prevRoot + "|" + receipt.ReceiptHash))
		expectedRoot := "b3:" + hex32(rootBytes[:])
		if receipt.Root != expectedRoot {
			return fmt.Errorf("receipt %d: root mismatch (expected %s, got %s)", i, expectedRoot, receipt.Root)
		}

		prevRoot = receipt.Root
	}

	// Check final root matches ROOT.txt
	rootPath := filepath.Join(ledgerDir, "ROOT.txt")
	rootData, err := os.ReadFile(rootPath)
	if err != nil {
		return fmt.Errorf("failed to read ROOT.txt: %w", err)
	}
	currentRoot := string(rootData)
	// Trim whitespace
	currentRoot = currentRoot[:len(currentRoot)-1]

	if currentRoot != prevRoot {
		return fmt.Errorf("ROOT.txt mismatch (expected %s, got %s)", prevRoot, currentRoot)
	}

	fmt.Printf("✓ All receipts verified successfully!\n")
	fmt.Printf("✓ Final root: %s\n", currentRoot)

	return nil
}

func hex32(b []byte) string {
	const hexDigits = "0123456789abcdef"
	var buf [64]byte
	for i := 0; i < 32; i++ {
		buf[i*2] = hexDigits[b[i]>>4]
		buf[i*2+1] = hexDigits[b[i]&0x0f]
	}
	return string(buf[:])
}
