// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timezone_test

import (
	"testing"

	timezone_module "code.gitea.io/gitea/modules/timezone"

	"github.com/stretchr/testify/assert"
)

func TestTimezoneOffsetString(t *testing.T) {
	timeZone := new(timezone_module.TimeZone)

	timeZone.Offset = 0
	assert.Equal(t, "00:00", timeZone.OffsetString())

	timeZone.Offset = 3600
	assert.Equal(t, "+01:00", timeZone.OffsetString())

	timeZone.Offset = -3600
	assert.Equal(t, "-01:00", timeZone.OffsetString())
}

func TestTimezoneIsEmpty(t *testing.T) {
	timeZone := new(timezone_module.TimeZone)

	assert.True(t, timeZone.IsEmpty())

	timeZone.Name = "Test"
	assert.False(t, timeZone.IsEmpty())
}
