// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestPushMirrorsIterate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	now := timeutil.TimeStampNow()

	InsertPushMirror(&PushMirror{
		RemoteName:     "test-1",
		LastUpdateUnix: now,
		Interval:       1,
	})

	long, _ := time.ParseDuration("24h")
	InsertPushMirror(&PushMirror{
		RemoteName:     "test-2",
		LastUpdateUnix: now,
		Interval:       long,
	})

	InsertPushMirror(&PushMirror{
		RemoteName:     "test-3",
		LastUpdateUnix: now,
		Interval:       0,
	})

	time.Sleep(1 * time.Millisecond)

	PushMirrorsIterate(func(idx int, bean interface{}) error {
		m, ok := bean.(*PushMirror)
		assert.True(t, ok)
		assert.Equal(t, "test-1", m.RemoteName)
		assert.Equal(t, m.RemoteName, m.GetRemoteName())
		return nil
	})
}
