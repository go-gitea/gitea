// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestDateTime(t *testing.T) {
	oldTz := setting.DefaultUILocation
	setting.DefaultUILocation, _ = time.LoadLocation("America/New_York")
	defer func() {
		setting.DefaultUILocation = oldTz
	}()

	refTimeStr := "2018-01-01T00:00:00Z"
	refTime, _ := time.Parse(time.RFC3339, refTimeStr)
	refTimeStamp := TimeStamp(refTime.Unix())

	assert.EqualValues(t, "-", DateTime("short", nil))
	assert.EqualValues(t, "-", DateTime("short", 0))
	assert.EqualValues(t, "-", DateTime("short", time.Time{}))
	assert.EqualValues(t, "-", DateTime("short", TimeStamp(0)))

	actual := DateTime("short", "invalid")
	assert.EqualValues(t, `<relative-time format="datetime" year="numeric" month="short" day="numeric" weekday="" datetime="invalid">invalid</relative-time>`, actual)

	actual = DateTime("short", refTimeStr)
	assert.EqualValues(t, `<relative-time format="datetime" year="numeric" month="short" day="numeric" weekday="" datetime="2018-01-01T00:00:00Z">2018-01-01T00:00:00Z</relative-time>`, actual)

	actual = DateTime("short", refTime)
	assert.EqualValues(t, `<relative-time format="datetime" year="numeric" month="short" day="numeric" weekday="" datetime="2018-01-01T00:00:00Z">2018-01-01</relative-time>`, actual)

	actual = DateTime("short", refTimeStamp)
	assert.EqualValues(t, `<relative-time format="datetime" year="numeric" month="short" day="numeric" weekday="" datetime="2017-12-31T19:00:00-05:00">2017-12-31</relative-time>`, actual)

	actual = DateTime("full", refTimeStamp)
	assert.EqualValues(t, `<relative-time format="datetime" weekday="" year="numeric" month="short" day="numeric" hour="numeric" minute="numeric" second="numeric" datetime="2017-12-31T19:00:00-05:00">2017-12-31 19:00:00 -05:00</relative-time>`, actual)
}

func TestParseDateTimeGraceful(t *testing.T) {
	testData := []struct {
		value    any
		expected any
		isFail   bool
	}{
		{
			value:    "2006-01-02T15:04:05Z",
			expected: int64(1136214245),
		},
		{
			value:    "2006-01-02T15:04:05.999Z",
			expected: int64(1136214245),
		},
		{
			value:    "2006-01-02T15:04:05-07:00",
			expected: int64(1136239445),
		},
		{
			value:    "2006-01-02T15:04:05.999-07:00",
			expected: int64(1136239445),
		},
		{
			value:    int64(1136214245),
			expected: int64(1136214245),
		},
		{
			value:    "1136214245",
			expected: int64(1136214245),
		},
		{
			value:    int64(1622040867000),
			expected: int64(-62135596800),
			isFail:   true,
		},
		{
			value:    "1622040867000",
			expected: int64(-62135596800),
			isFail:   true,
		},
		{
			value:    0,
			expected: int64(-62135596800),
			isFail:   true,
		},
		{
			value:    nil,
			expected: int64(-62135596800),
			isFail:   true,
		},
		{
			value:    "",
			expected: int64(-62135596800),
			isFail:   true,
		},
	}

	for _, testCase := range testData {
		actual, err := ParseDateTimeGraceful(testCase.value)

		if testCase.isFail == true {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
		}

		assert.EqualValues(t, testCase.expected, actual.Unix())
	}
}
