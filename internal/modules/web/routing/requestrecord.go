// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"net/http"
	"sync"
	"time"
)

type requestRecord struct {
	// index of the record in the records map
	index uint64

	// immutable fields
	startTime      time.Time
	request        *http.Request
	responseWriter http.ResponseWriter

	// mutex
	lock sync.RWMutex

	// mutable fields
	isLongPolling bool
	funcInfo      *FuncInfo
	panicError    any
}
