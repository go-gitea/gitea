// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sync

import (
	"testing"
)

func Test_Lock(t *testing.T) {
	locker := GetLock("test")
	locker.Lock()
	locker.Unlock()
}
