// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestDateTime(t *testing.T) {
	testTz, _ := time.LoadLocation("America/New_York")
	defer test.MockVariableValue(&setting.DefaultUILocation, testTz)()

	refTimeStr := "2018-01-01T00:00:00Z"
	refDateStr := "2018-01-01"
	refTime, _ := time.Parse(time.RFC3339, refTimeStr)
	refTimeStamp := TimeStamp(refTime.Unix())

	assert.EqualValues(t, "-", DateTime("short", nil))
	assert.EqualValues(t, "-", DateTime("short", 0))
	assert.EqualValues(t, "-", DateTime("short", time.Time{}))
	assert.EqualValues(t, "-", DateTime("short", TimeStamp(0)))

	actual := DateTime("short", "invalid")
	assert.EqualValues(t, `<gitea-absolute-date weekday="" year="numeric" month="short" day="numeric" date="invalid">invalid</gitea-absolute-date>`, actual)

	actual = DateTime("short", refTimeStr)
	assert.EqualValues(t, `<gitea-absolute-date weekday="" year="numeric" month="short" day="numeric" date="2018-01-01T00:00:00Z">2018-01-01T00:00:00Z</gitea-absolute-date>`, actual)

	actual = DateTime("short", refTime)
	assert.EqualValues(t, `<gitea-absolute-date weekday="" year="numeric" month="short" day="numeric" date="2018-01-01T00:00:00Z">2018-01-01</gitea-absolute-date>`, actual)

	actual = DateTime("short", refDateStr)
	assert.EqualValues(t, `<gitea-absolute-date weekday="" year="numeric" month="short" day="numeric" date="2018-01-01">2018-01-01</gitea-absolute-date>`, actual)

	actual = DateTime("short", refTimeStamp)
	assert.EqualValues(t, `<gitea-absolute-date weekday="" year="numeric" month="short" day="numeric" date="2017-12-31T19:00:00-05:00">2017-12-31</gitea-absolute-date>`, actual)

	actual = DateTime("full", refTimeStamp)
	assert.EqualValues(t, `<relative-time weekday="" year="numeric" format="datetime" month="short" day="numeric" hour="numeric" minute="numeric" second="numeric" data-tooltip-content data-tooltip-interactive="true" datetime="2017-12-31T19:00:00-05:00">2017-12-31 19:00:00 -05:00</relative-time>`, actual)
}
