// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"net/http"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
)

type requestRecord struct {
	// immutable fields
	index      uint64 // unique number (per process) for the request
	startTime  time.Time
	request    *http.Request
	respWriter http.ResponseWriter

	// mutex
	lock sync.RWMutex

	// below are mutable fields
	funcInfo *FuncInfo
	// * for "mark as long polling"
	isLongPolling bool
	// * for router logger
	logLevel   log.Level
	panicError error
}
