package base

import (
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"github.com/Unknwon/i18n"
	macaroni18n "github.com/go-macaron/i18n"
	"github.com/stretchr/testify/assert"
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

func TestEncodeMD5(t *testing.T) {
	assert.Equal(t,
		"3858f62230ac3c915f300c664312c63f",
		EncodeMD5("foobar"),
	)
}

func TestEncodeSha1(t *testing.T) {
	assert.Equal(t,
		"8843d7f92416211de9ebb963ff4ce28125932878",
		EncodeSha1("foobar"),
	)
}

func TestShortSha(t *testing.T) {
	assert.Equal(t, "veryverylo", ShortSha("veryverylong"))
}

func TestDetectEncoding(t *testing.T) {
	testSuccess := func(b []byte, expected string) {
		encoding, err := DetectEncoding(b)
		assert.NoError(t, err)
		assert.Equal(t, expected, encoding)
	}
	// utf-8
	b := []byte("just some ascii")
	testSuccess(b, "UTF-8")

	// utf-8-sig: "hey" (with BOM)
	b = []byte{0xef, 0xbb, 0xbf, 0x68, 0x65, 0x79}
	testSuccess(b, "UTF-8")

	// utf-16: "hey<accented G>"
	b = []byte{0xff, 0xfe, 0x68, 0x00, 0x65, 0x00, 0x79, 0x00, 0xf4, 0x01}
	testSuccess(b, "UTF-16LE")

	// iso-8859-1: d<accented e>cor<newline>
	b = []byte{0x44, 0xe9, 0x63, 0x6f, 0x72, 0x0a}
	encoding, err := DetectEncoding(b)
	assert.NoError(t, err)
	// due to a race condition in `chardet` library, it could either detect
	// "ISO-8859-1" or "IS0-8859-2" here. Technically either is correct, so
	// we accept either.
	assert.Contains(t, encoding, "ISO-8859")

	setting.Repository.AnsiCharset = "placeholder"
	testSuccess(b, "placeholder")

	// invalid bytes
	b = []byte{0xfa}
	_, err = DetectEncoding(b)
	assert.Error(t, err)
}

func TestBasicAuthDecode(t *testing.T) {
	_, _, err := BasicAuthDecode("?")
	assert.Equal(t, "illegal base64 data at input byte 0", err.Error())

	user, pass, err := BasicAuthDecode("Zm9vOmJhcg==")
	assert.NoError(t, err)
	assert.Equal(t, "foo", user)
	assert.Equal(t, "bar", pass)
}

func TestBasicAuthEncode(t *testing.T) {
	assert.Equal(t, "Zm9vOmJhcg==", BasicAuthEncode("foo", "bar"))
}

func TestGetRandomString(t *testing.T) {
	randomString, err := GetRandomString(4)
	assert.NoError(t, err)
	assert.Len(t, randomString, 4)
}

// TODO: Test PBKDF2()
// TODO: Test VerifyTimeLimitCode()
// TODO: Test CreateTimeLimitCode()

func TestHashEmail(t *testing.T) {
	assert.Equal(t,
		"d41d8cd98f00b204e9800998ecf8427e",
		HashEmail(""),
	)
	assert.Equal(t,
		"353cbad9b58e69c96154ad99f92bedc7",
		HashEmail("gitea@example.com"),
	)
}

func TestAvatarLink(t *testing.T) {
	setting.EnableFederatedAvatar = false
	setting.LibravatarService = nil
	setting.DisableGravatar = true

	assert.Equal(t, "/img/avatar_default.png", AvatarLink(""))

	setting.DisableGravatar = false
	assert.Equal(t,
		"353cbad9b58e69c96154ad99f92bedc7",
		AvatarLink("gitea@example.com"),
	)
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

func TestFileSize(t *testing.T) {
	var size int64
	size = 512
	assert.Equal(t, "512B", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512KB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512MB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512GB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512TB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512PB", FileSize(size))
	size = size * 4
	assert.Equal(t, "2.0EB", FileSize(size))
}

func TestSubtract(t *testing.T) {
	toFloat64 := func(n interface{}) float64 {
		switch n.(type) {
		case int:
			return float64(n.(int))
		case int8:
			return float64(n.(int8))
		case int16:
			return float64(n.(int16))
		case int32:
			return float64(n.(int32))
		case int64:
			return float64(n.(int64))
		case float32:
			return float64(n.(float32))
		case float64:
			return n.(float64)
		default:
			return 0.0
		}
	}
	values := []interface{}{
		int(-3),
		int8(14),
		int16(81),
		int32(-156),
		int64(1528),
		float32(3.5),
		float64(-15.348),
	}
	for _, left := range values {
		for _, right := range values {
			expected := toFloat64(left) - toFloat64(right)
			sub := Subtract(left, right)
			assert.InDelta(t, expected, sub, 1e-3)
		}
	}
}

func TestEllipsisString(t *testing.T) {
	assert.Equal(t, "...", EllipsisString("foobar", 0))
	assert.Equal(t, "...", EllipsisString("foobar", 1))
	assert.Equal(t, "...", EllipsisString("foobar", 2))
	assert.Equal(t, "...", EllipsisString("foobar", 3))
	assert.Equal(t, "f...", EllipsisString("foobar", 4))
	assert.Equal(t, "fo...", EllipsisString("foobar", 5))
	assert.Equal(t, "foobar", EllipsisString("foobar", 6))
	assert.Equal(t, "foobar", EllipsisString("foobar", 10))
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "", TruncateString("foobar", 0))
	assert.Equal(t, "f", TruncateString("foobar", 1))
	assert.Equal(t, "fo", TruncateString("foobar", 2))
	assert.Equal(t, "foo", TruncateString("foobar", 3))
	assert.Equal(t, "foob", TruncateString("foobar", 4))
	assert.Equal(t, "fooba", TruncateString("foobar", 5))
	assert.Equal(t, "foobar", TruncateString("foobar", 6))
	assert.Equal(t, "foobar", TruncateString("foobar", 7))
}

func TestStringsToInt64s(t *testing.T) {
	testSuccess := func(input []string, expected []int64) {
		result, err := StringsToInt64s(input)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	}
	testSuccess([]string{}, []int64{})
	testSuccess([]string{"-1234"}, []int64{-1234})
	testSuccess([]string{"1", "4", "16", "64", "256"},
		[]int64{1, 4, 16, 64, 256})

	_, err := StringsToInt64s([]string{"-1", "a", "$"})
	assert.Error(t, err)
}

func TestInt64sToStrings(t *testing.T) {
	assert.Equal(t, []string{}, Int64sToStrings([]int64{}))
	assert.Equal(t,
		[]string{"1", "4", "16", "64", "256"},
		Int64sToStrings([]int64{1, 4, 16, 64, 256}),
	)
}

func TestInt64sToMap(t *testing.T) {
	assert.Equal(t, map[int64]bool{}, Int64sToMap([]int64{}))
	assert.Equal(t,
		map[int64]bool{1: true, 4: true, 16: true},
		Int64sToMap([]int64{1, 4, 16}),
	)
}

func TestIsLetter(t *testing.T) {
	assert.True(t, IsLetter('a'))
	assert.True(t, IsLetter('e'))
	assert.True(t, IsLetter('q'))
	assert.True(t, IsLetter('z'))
	assert.True(t, IsLetter('A'))
	assert.True(t, IsLetter('E'))
	assert.True(t, IsLetter('Q'))
	assert.True(t, IsLetter('Z'))
	assert.True(t, IsLetter('_'))
	assert.False(t, IsLetter('-'))
	assert.False(t, IsLetter('1'))
	assert.False(t, IsLetter('$'))
}

func TestIsTextFile(t *testing.T) {
	assert.True(t, IsTextFile([]byte{}))
	assert.True(t, IsTextFile([]byte("lorem ipsum")))
}

// TODO: IsImageFile(), currently no idea how to test
// TODO: IsPDFFile(), currently no idea how to test
