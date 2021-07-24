// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"time"
)

// StopTimer is a utility function to safely stop a time.Timer and clean its channel
func StopTimer(t *time.Timer) bool {
	stopped := t.Stop()
	if !stopped {
		select {
		case <-t.C:
		default:
		}
	}
	return stopped
}
