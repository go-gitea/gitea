// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecToTime(t *testing.T) {
	assert.Equal(t, SecToTime(10), "10s")
	assert.Equal(t, SecToTime(100), "1m 40s")
	assert.Equal(t, SecToTime(1000), "16m 40s")
	assert.Equal(t, SecToTime(10000), "2h 46m 40s")
	assert.Equal(t, SecToTime(100000), "1d 3h 46m 40s")
	assert.Equal(t, SecToTime(1000000), "11d 13h 46m 40s")
}
