// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/modules/gitvm/ledger"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// GetRoot returns the current GitVM root hash as JSON
func GetRoot(w http.ResponseWriter, r *http.Request) {
	l := ledger.New(setting.GitVM.Dir)
	root, err := l.GetRoot()
	if err != nil {
		log.Error("GitVM: failed to get root: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if root == "" {
		w.Write([]byte(`{"root":""}`))
		return
	}
	w.Write([]byte(`{"root":"` + root + `"}`))
}

// GetRootPlainText returns the current GitVM root hash as plain text
func GetRootPlainText(w http.ResponseWriter, r *http.Request) {
	l := ledger.New(setting.GitVM.Dir)
	root, err := l.GetRoot()
	if err != nil {
		log.Error("GitVM: failed to get root: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	if root == "" {
		w.Write([]byte("(no receipts yet)\n"))
		return
	}
	w.Write([]byte(root + "\n"))
}

// GetReceipts returns recent receipts (optionally paginated)
func GetReceipts(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 200 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	l := ledger.New(setting.GitVM.Dir)
	receipts, err := l.ReadReceipts()
	if err != nil {
		log.Error("GitVM: failed to read receipts: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// return the last N receipts
	start := 0
	if len(receipts) > limit {
		start = len(receipts) - limit
	}
	receipts = receipts[start:]

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(receipts); err != nil {
		log.Error("GitVM: failed to encode receipts: %v", err)
	}
}

// GetReceiptsNDJSON streams receipts as NDJSON with cursor-based pagination
func GetReceiptsNDJSON(w http.ResponseWriter, r *http.Request) {
	// Parse query params
	var opts ledger.ReceiptSliceOptions

	// cursor (byte offset)
	if cursorStr := r.URL.Query().Get("cursor"); cursorStr != "" {
		if c, err := strconv.ParseInt(cursorStr, 10, 64); err == nil && c >= 0 {
			opts.Cursor = c
		}
	}

	// since_ts (unix milliseconds, inclusive)
	if sinceTS := r.URL.Query().Get("since_ts"); sinceTS != "" {
		if ts, err := strconv.ParseInt(sinceTS, 10, 64); err == nil && ts >= 0 {
			opts.SinceTS = ts
		}
	}

	// since_root (root hash, exclusive)
	opts.SinceRoot = r.URL.Query().Get("since_root")

	// limit (max receipts, default 1000, max 20000)
	opts.Limit = 1000
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > 20000 {
				l = 20000
			}
			opts.Limit = l
		}
	}

	// Read slice
	l := ledger.New(setting.GitVM.Dir)
	result, err := l.ReadReceiptSlice(opts)
	if err != nil {
		log.Error("GitVM: failed to read receipt slice: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get current root for X-GitVM-Root header
	root, err := l.GetRoot()
	if err != nil {
		log.Error("GitVM: failed to get root: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-GitVM-Root", root)
	w.Header().Set("X-GitVM-Cursor", strconv.FormatInt(result.NextCursor, 10))

	// Stream NDJSON lines
	for _, line := range result.Lines {
		w.Write([]byte(line + "\n"))
	}
}
