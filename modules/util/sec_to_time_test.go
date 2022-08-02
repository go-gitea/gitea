// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecToTime(t *testing.T) {
	assert.Equal(t, "1 minute 6 seconds", SecToTime(66))
	assert.Equal(t, "1 hour", SecToTime(3600))
	assert.Equal(t, "1 hour", SecToTime(3601))
	assert.Equal(t, "14 hours 33 minutes", SecToTime(52410))
	assert.Equal(t, "6 days 12 hours", SecToTime(563418))
	assert.Equal(t, "2 weeks 4 days", SecToTime(1563418))
	assert.Equal(t, "4 weeks", SecToTime(2419200))
	assert.Equal(t, "4 weeks 1 day", SecToTime(2505600))
	assert.Equal(t, "1 month 2 weeks", SecToTime(3937125))
	assert.Equal(t, "11 months 1 week", SecToTime(29376000))
	assert.Equal(t, "1 year 5 months", SecToTime(45677465))
}
