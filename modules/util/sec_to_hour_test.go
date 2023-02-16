// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecToHour(t *testing.T) {
	// Less than a minute
	assert.Equal(t, SecToHour(56), "")
	// Singular minute
	assert.Equal(t, SecToHour(66), "1 minute")
	// Plural minutes
	assert.Equal(t, SecToHour(300), "5 minutes")
	// Singular hour
	assert.Equal(t, SecToHour(3600), "1 hour")
	// Singular hour + minute
	assert.Equal(t, SecToHour(3660), "1 hour 1 minute")
	// Singular hour, plural minute
	assert.Equal(t, SecToHour(4199), "1 hour 9 minutes")
	// Rounding (lack of)
	assert.Equal(t, SecToHour(4200), "1 hour 10 minutes")
	assert.Equal(t, SecToHour(4259), "1 hour 10 minutes")
	// Going over days, weeks and still showing only hours
	assert.Equal(t, SecToHour(52410), "14 hours 33 minutes")
	assert.Equal(t, SecToHour(563418), "156 hours 30 minutes")
	assert.Equal(t, SecToHour(1563418), "434 hours 16 minutes")
}
