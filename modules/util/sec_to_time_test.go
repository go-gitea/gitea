// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecToTime(t *testing.T) {
	assert.Equal(t, SecToTime(66), "1 minute 6 seconds")
	assert.Equal(t, SecToTime(52410), "14 hours 33 minutes")
	assert.Equal(t, SecToTime(563418), "6 days 12 hours")
	assert.Equal(t, SecToTime(1563418), "2 weeks 4 days")
	assert.Equal(t, SecToTime(3937125), "1 month 2 weeks")
	assert.Equal(t, SecToTime(45677465), "1 year 5 months")
}
