// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import "unsafe"

//go:linkname runtime_getProfLabel runtime/pprof.runtime_getProfLabel
func runtime_getProfLabel() unsafe.Pointer // nolint

type labelMap map[string]string

func getGoroutineLabels() map[string]string {
	l := (*labelMap)(runtime_getProfLabel())
	if l == nil {
		return nil
	}
	return *l
}
