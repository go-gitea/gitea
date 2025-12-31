// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

// createTestFixture creates a test receipts.jsonl with 10 sample receipts
func createTestFixture(t *testing.T, dir string) {
	t.Helper()

	// Sample receipts with incrementing timestamps and different roots
	// These are minimal valid receipts for testing slicing logic
	receipts := []string{
		`{"v":1,"type":"git.push","ts_unix_ms":1000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"ref":"refs/heads/main","before":"a","after":"b"},"prev_root":"","receipt_hash":"b3:root1hash","root":"b3:root1"}`,
		`{"v":1,"type":"git.push","ts_unix_ms":2000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"ref":"refs/heads/main","before":"b","after":"c"},"prev_root":"b3:root1","receipt_hash":"b3:root2hash","root":"b3:root2"}`,
		`{"v":1,"type":"pr.merged","ts_unix_ms":3000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":2,"username":"user2"},"payload":{"pr_id":1,"merge_commit":"c"},"prev_root":"b3:root2","receipt_hash":"b3:root3hash","root":"b3:root3"}`,
		`{"v":1,"type":"ci.run.start","ts_unix_ms":4000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"run_id":1,"commit_sha":"c","workflow_id":"test.yml"},"prev_root":"b3:root3","receipt_hash":"b3:root4hash","root":"b3:root4"}`,
		`{"v":1,"type":"ci.run.end","ts_unix_ms":5000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"run_id":1,"status":"success","duration_ms":1000,"commit_sha":"c","workflow_id":"test.yml","ref":"refs/heads/main","event":"push"},"prev_root":"b3:root4","receipt_hash":"b3:root5hash","root":"b3:root5"}`,
		`{"v":1,"type":"git.push","ts_unix_ms":6000000,"repo":{"id":2,"full":"test/repo2"},"actor":{"id":3,"username":"user3"},"payload":{"ref":"refs/heads/dev","before":"x","after":"y"},"prev_root":"b3:root5","receipt_hash":"b3:root6hash","root":"b3:root6"}`,
		`{"v":1,"type":"release.published","ts_unix_ms":7000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"tag":"v1.0.0"},"prev_root":"b3:root6","receipt_hash":"b3:root7hash","root":"b3:root7"}`,
		`{"v":1,"type":"perm.changed","ts_unix_ms":8000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"target_user_id":4,"mode":"write"},"prev_root":"b3:root7","receipt_hash":"b3:root8hash","root":"b3:root8"}`,
		`{"v":1,"type":"git.push","ts_unix_ms":9000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"ref":"refs/heads/main","before":"y","after":"z"},"prev_root":"b3:root8","receipt_hash":"b3:root9hash","root":"b3:root9"}`,
		`{"v":1,"type":"ci.run.end","ts_unix_ms":10000000,"repo":{"id":1,"full":"test/repo1"},"actor":{"id":1,"username":"user1"},"payload":{"run_id":2,"status":"failure","duration_ms":2000,"commit_sha":"z","workflow_id":"test.yml","ref":"refs/heads/main","event":"push"},"prev_root":"b3:root9","receipt_hash":"b3:root10hash","root":"b3:root10"}`,
	}

	content := ""
	for _, r := range receipts {
		content += r + "\n"
	}

	if err := os.WriteFile(filepath.Join(dir, "receipts.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test fixture: %v", err)
	}
}

func TestReadReceiptSlice_CursorPagination(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Test 1: Read first 3 receipts
	result1, err := l.ReadReceiptSlice(ReceiptSliceOptions{Limit: 3})
	if err != nil {
		t.Fatalf("failed to read first slice: %v", err)
	}
	if len(result1.Lines) != 3 {
		t.Errorf("expected 3 receipts, got %d", len(result1.Lines))
	}
	if result1.FileSize == 0 {
		t.Error("FileSize should be set")
	}

	// Test 2: Read next 3 using cursor
	result2, err := l.ReadReceiptSlice(ReceiptSliceOptions{
		Cursor: result1.NextCursor,
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("failed to read second slice: %v", err)
	}
	if len(result2.Lines) != 3 {
		t.Errorf("expected 3 receipts, got %d", len(result2.Lines))
	}

	// Test 3: Ensure no duplicates between pages
	if result1.Lines[2] == result2.Lines[0] {
		t.Error("cursor pagination produced duplicates")
	}

	// Test 4: Ensure cursors advance
	if result2.NextCursor <= result1.NextCursor {
		t.Errorf("cursor did not advance: %d -> %d", result1.NextCursor, result2.NextCursor)
	}
}

func TestReadReceiptSlice_SinceRoot(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Read receipts after root3 (exclusive)
	result, err := l.ReadReceiptSlice(ReceiptSliceOptions{
		SinceRoot: "b3:root3",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("failed to read slice: %v", err)
	}

	// Should get receipts 4-10 (7 receipts after root3)
	if len(result.Lines) != 7 {
		t.Errorf("expected 7 receipts after root3, got %d", len(result.Lines))
	}

	// First receipt should have root4
	if result.Lines[0] == "" || len(result.Lines[0]) < 10 {
		t.Error("unexpected receipt format")
	}
}

func TestReadReceiptSlice_UntilRoot(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Read receipts until root5 (exclusive)
	result, err := l.ReadReceiptSlice(ReceiptSliceOptions{
		UntilRoot: "b3:root5",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("failed to read slice: %v", err)
	}

	// Should get receipts 1-4 (stops before root5)
	if len(result.Lines) != 4 {
		t.Errorf("expected 4 receipts until root5, got %d", len(result.Lines))
	}
}

func TestReadReceiptSlice_SinceTS(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Read receipts from timestamp 5000000 onwards (inclusive)
	result, err := l.ReadReceiptSlice(ReceiptSliceOptions{
		SinceTS: 5000000,
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("failed to read slice: %v", err)
	}

	// Should get receipts with ts >= 5000000 (receipts 5-10 = 6 receipts)
	if len(result.Lines) != 6 {
		t.Errorf("expected 6 receipts since ts=5000000, got %d", len(result.Lines))
	}
}

func TestReadReceiptSlice_UntilTS(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Read receipts until timestamp 5000000 (inclusive)
	result, err := l.ReadReceiptSlice(ReceiptSliceOptions{
		UntilTS: 5000000,
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("failed to read slice: %v", err)
	}

	// Should get receipts with ts <= 5000000 (receipts 1-5 = 5 receipts)
	if len(result.Lines) != 5 {
		t.Errorf("expected 5 receipts until ts=5000000, got %d", len(result.Lines))
	}
}

func TestReadReceiptSlice_TimeRange(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Read receipts in time range [4000000, 7000000]
	result, err := l.ReadReceiptSlice(ReceiptSliceOptions{
		SinceTS: 4000000,
		UntilTS: 7000000,
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("failed to read slice: %v", err)
	}

	// Should get receipts 4-7 (4 receipts)
	if len(result.Lines) != 4 {
		t.Errorf("expected 4 receipts in time range, got %d", len(result.Lines))
	}
}

func TestReadReceiptSlice_RootRange(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Read receipts between root2 (exclusive) and root6 (exclusive)
	result, err := l.ReadReceiptSlice(ReceiptSliceOptions{
		SinceRoot: "b3:root2",
		UntilRoot: "b3:root6",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("failed to read slice: %v", err)
	}

	// Should get receipts 3-5 (3 receipts after root2, before root6)
	if len(result.Lines) != 3 {
		t.Errorf("expected 3 receipts in root range, got %d", len(result.Lines))
	}
}

func TestReadReceiptSlice_EmptyFile(t *testing.T) {
	dir := t.TempDir()

	l := &Ledger{Dir: dir}

	// Read from non-existent file
	result, err := l.ReadReceiptSlice(ReceiptSliceOptions{Limit: 100})
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if len(result.Lines) != 0 {
		t.Error("expected empty result for missing file")
	}
	if result.NextCursor != 0 {
		t.Error("expected cursor 0 for empty file")
	}
	if result.FileSize != 0 {
		t.Error("expected file size 0 for missing file")
	}
}

func TestReadReceiptSlice_FileSizeConsistency(t *testing.T) {
	dir := t.TempDir()
	createTestFixture(t, dir)

	l := &Ledger{Dir: dir}

	// Multiple requests should see same file size (forensic anchor)
	result1, _ := l.ReadReceiptSlice(ReceiptSliceOptions{Limit: 5})
	result2, _ := l.ReadReceiptSlice(ReceiptSliceOptions{Cursor: result1.NextCursor, Limit: 5})

	if result1.FileSize != result2.FileSize {
		t.Errorf("file size should be consistent: %d vs %d", result1.FileSize, result2.FileSize)
	}
}
