// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecToTime(t *testing.T) {
	second := int64(1)
	minute := 60 * second
	hour := 60 * minute
	day := 24 * hour
	year := 365 * day

	assert.Equal(t, "1 minute 6 seconds", SecToTime(minute+6*second))
	assert.Equal(t, "1 hour", SecToTime(hour))
	assert.Equal(t, "1 hour", SecToTime(hour+second))
	assert.Equal(t, "14 hours 33 minutes", SecToTime(14*hour+33*minute+30*second))
	assert.Equal(t, "6 days 12 hours", SecToTime(6*day+12*hour+30*minute+18*second))
	assert.Equal(t, "2 weeks 4 days", SecToTime((2*7+4)*day+2*hour+16*minute+58*second))
	assert.Equal(t, "4 weeks", SecToTime(4*7*day))
	assert.Equal(t, "4 weeks 1 day", SecToTime((4*7+1)*day))
	assert.Equal(t, "1 month 2 weeks", SecToTime((6*7+3)*day+13*hour+38*minute+45*second))
	assert.Equal(t, "11 months", SecToTime(year-25*day))
	assert.Equal(t, "1 year 5 months", SecToTime(year+163*day+10*hour+11*minute+5*second))
}
