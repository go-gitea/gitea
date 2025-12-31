// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package http

import (
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers GitVM HTTP endpoints
func RegisterRoutes(r chi.Router) {
	r.Get("/gitvm/root", GetRoot)
	r.Get("/gitvm/receipts", GetReceipts)
}
