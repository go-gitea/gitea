// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import "runtime"

func CallerFuncName(optSkipParent ...int) string {
	pc := make([]uintptr, 1)
	skipParent := 0
	if len(optSkipParent) > 0 {
		skipParent = optSkipParent[0]
	}
	runtime.Callers(skipParent+1 /*this*/ +1 /*runtime*/, pc)
	funcName := runtime.FuncForPC(pc[0]).Name()
	return funcName
}
