// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecToHours(t *testing.T) {
	second := int64(1)
	minute := 60 * second
	hour := 60 * minute
	day := 24 * hour

	assert.Equal(t, "1 minute", SecToHours(minute+6*second))
	assert.Equal(t, "1 hour", SecToHours(hour))
	assert.Equal(t, "1 hour", SecToHours(hour+second))
	assert.Equal(t, "14 hours 33 minutes", SecToHours(14*hour+33*minute+30*second))
	assert.Equal(t, "156 hours 30 minutes", SecToHours(6*day+12*hour+30*minute+18*second))
	assert.Equal(t, "98 hours 16 minutes", SecToHours(4*day+2*hour+16*minute+58*second))
	assert.Equal(t, "672 hours", SecToHours(4*7*day))
	assert.Equal(t, "1 second", SecToHours(1))
	assert.Equal(t, "2 seconds", SecToHours(2))
	assert.Equal(t, "", SecToHours(nil)) // old behavior, empty means no output
}
