// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import "runtime"

func CallerFuncName(skip int) string {
	pc := make([]uintptr, 1)
	runtime.Callers(skip+1, pc)
	funcName := runtime.FuncForPC(pc[0]).Name()
	return funcName
}
