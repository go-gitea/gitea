// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package timeutil

import (
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

	macaroni18n "gitea.com/macaron/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

var BaseDate time.Time

// time durations
const (
	DayDur   = 24 * time.Hour
	WeekDur  = 7 * DayDur
	MonthDur = 30 * DayDur
	YearDur  = 12 * MonthDur
)

func TestMain(m *testing.M) {
	// setup
	macaroni18n.I18n(macaroni18n.Options{
		Directory:   "../../options/locale/",
		DefaultLang: "en-US",
		Langs:       []string{"en-US"},
		Names:       []string{"english"},
	})
	BaseDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

	// run the tests
	retVal := m.Run()

	os.Exit(retVal)
}

func TestTimeSince(t *testing.T) {
	assert.Equal(t, "now", timeSince(BaseDate, BaseDate, "en"))

	// test that each diff in `diffs` yields the expected string
	test := func(expected string, diffs ...time.Duration) {
		for _, diff := range diffs {
			actual := timeSince(BaseDate, BaseDate.Add(diff), "en")
			assert.Equal(t, i18n.Tr("en", "tool.ago", expected), actual)
			actual = timeSince(BaseDate.Add(diff), BaseDate, "en")
			assert.Equal(t, i18n.Tr("en", "tool.from_now", expected), actual)
		}
	}
	test("1 second", time.Second, time.Second+50*time.Millisecond)
	test("2 seconds", 2*time.Second, 2*time.Second+50*time.Millisecond)
	test("1 minute", time.Minute, time.Minute+30*time.Second)
	test("2 minutes", 2*time.Minute, 2*time.Minute+30*time.Second)
	test("1 hour", time.Hour, time.Hour+30*time.Minute)
	test("2 hours", 2*time.Hour, 2*time.Hour+30*time.Minute)
	test("1 day", DayDur, DayDur+12*time.Hour)
	test("2 days", 2*DayDur, 2*DayDur+12*time.Hour)
	test("1 week", WeekDur, WeekDur+3*DayDur)
	test("2 weeks", 2*WeekDur, 2*WeekDur+3*DayDur)
	test("1 month", MonthDur, MonthDur+15*DayDur)
	test("2 months", 2*MonthDur, 2*MonthDur+15*DayDur)
	test("1 year", YearDur, YearDur+6*MonthDur)
	test("2 years", 2*YearDur, 2*YearDur+6*MonthDur)
}

func TestTimeSincePro(t *testing.T) {
	assert.Equal(t, "now", timeSincePro(BaseDate, BaseDate, "en"))

	// test that a difference of `diff` yields the expected string
	test := func(expected string, diff time.Duration) {
		actual := timeSincePro(BaseDate, BaseDate.Add(diff), "en")
		assert.Equal(t, expected, actual)
		assert.Equal(t, "future", timeSincePro(BaseDate.Add(diff), BaseDate, "en"))
	}
	test("1 second", time.Second)
	test("2 seconds", 2*time.Second)
	test("1 minute", time.Minute)
	test("1 minute, 1 second", time.Minute+time.Second)
	test("1 minute, 59 seconds", time.Minute+59*time.Second)
	test("2 minutes", 2*time.Minute)
	test("1 hour", time.Hour)
	test("1 hour, 1 second", time.Hour+time.Second)
	test("1 hour, 59 minutes, 59 seconds", time.Hour+59*time.Minute+59*time.Second)
	test("2 hours", 2*time.Hour)
	test("1 day", DayDur)
	test("1 day, 23 hours, 59 minutes, 59 seconds",
		DayDur+23*time.Hour+59*time.Minute+59*time.Second)
	test("2 days", 2*DayDur)
	test("1 week", WeekDur)
	test("2 weeks", 2*WeekDur)
	test("1 month", MonthDur)
	test("3 months", 3*MonthDur)
	test("1 year", YearDur)
	test("2 years, 3 months, 1 week, 2 days, 4 hours, 12 minutes, 17 seconds",
		2*YearDur+3*MonthDur+WeekDur+2*DayDur+4*time.Hour+
			12*time.Minute+17*time.Second)
}

func TestHtmlTimeSince(t *testing.T) {
	setting.TimeFormat = time.UnixDate
	setting.DefaultUILocation = time.UTC
	// test that `diff` yields a result containing `expected`
	test := func(expected string, diff time.Duration) {
		actual := htmlTimeSince(BaseDate, BaseDate.Add(diff), "en")
		assert.Contains(t, actual, `title="Sat Jan  1 00:00:00 UTC 2000"`)
		assert.Contains(t, actual, expected)
	}
	test("1 second", time.Second)
	test("3 minutes", 3*time.Minute+5*time.Second)
	test("1 day", DayDur+18*time.Hour)
	test("1 week", WeekDur+6*DayDur)
	test("3 months", 3*MonthDur+3*WeekDur)
	test("2 years", 2*YearDur)
	test("3 years", 3*YearDur+11*MonthDur+4*WeekDur)
}

func TestComputeTimeDiff(t *testing.T) {
	// test that for each offset in offsets,
	// computeTimeDiff(base + offset) == (offset, str)
	test := func(base int64, str string, offsets ...int64) {
		for _, offset := range offsets {
			diff, diffStr := computeTimeDiff(base+offset, "en")
			assert.Equal(t, offset, diff)
			assert.Equal(t, str, diffStr)
		}
	}
	test(0, "now", 0)
	test(1, "1 second", 0)
	test(2, "2 seconds", 0)
	test(Minute, "1 minute", 0, 1, 30, Minute-1)
	test(2*Minute, "2 minutes", 0, Minute-1)
	test(Hour, "1 hour", 0, 1, Hour-1)
	test(5*Hour, "5 hours", 0, Hour-1)
	test(Day, "1 day", 0, 1, Day-1)
	test(5*Day, "5 days", 0, Day-1)
	test(Week, "1 week", 0, 1, Week-1)
	test(3*Week, "3 weeks", 0, 4*Day+25000)
	test(Month, "1 month", 0, 1, Month-1)
	test(10*Month, "10 months", 0, Month-1)
	test(Year, "1 year", 0, Year-1)
	test(3*Year, "3 years", 0, Year-1)
}

func TestMinutesToFriendly(t *testing.T) {
	// test that a number of minutes yields the expected string
	test := func(expected string, minutes int) {
		actual := MinutesToFriendly(minutes, "en")
		assert.Equal(t, expected, actual)
	}
	test("1 minute", 1)
	test("2 minutes", 2)
	test("1 hour", 60)
	test("1 hour, 1 minute", 61)
	test("1 hour, 2 minutes", 62)
	test("2 hours", 120)
}
