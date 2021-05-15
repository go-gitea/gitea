// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func TestEncodeSha256(t *testing.T) {
	assert.Equal(t,
		"c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2",
		EncodeSha256("foobar"),
	)
}

func TestShortSha(t *testing.T) {
	assert.Equal(t, "veryverylo", ShortSha("veryverylong"))
}

func TestBasicAuthDecode(t *testing.T) {
	_, _, err := BasicAuthDecode("?")
	assert.Equal(t, "illegal base64 data at input byte 0", err.Error())

	user, pass, err := BasicAuthDecode("Zm9vOmJhcg==")
	assert.NoError(t, err)
	assert.Equal(t, "foo", user)
	assert.Equal(t, "bar", pass)

	_, _, err = BasicAuthDecode("aW52YWxpZA==")
	assert.Error(t, err)

	_, _, err = BasicAuthDecode("invalid")
	assert.Error(t, err)
}

func TestBasicAuthEncode(t *testing.T) {
	assert.Equal(t, "Zm9vOmJhcg==", BasicAuthEncode("foo", "bar"))
	assert.Equal(t, "MjM6IjotLS0t", BasicAuthEncode("23:\"", "----"))
}

func TestVerifyTimeLimitCode(t *testing.T) {
	tc := []struct {
		data    string
		minutes int
		code    string
		valid   bool
	}{{
		data:    "data",
		minutes: 2,
		code:    testCreateTimeLimitCode(t, "data", 2),
		valid:   true,
	}, {
		data:    "abc123-ß",
		minutes: 1,
		code:    testCreateTimeLimitCode(t, "abc123-ß", 1),
		valid:   true,
	}, {
		data:    "data",
		minutes: 2,
		code:    "2021012723240000005928251dac409d2c33a6eb82c63410aaad569bed",
		valid:   false,
	}}
	for _, test := range tc {
		actualValid := VerifyTimeLimitCode(test.data, test.minutes, test.code)
		assert.Equal(t, test.valid, actualValid, "data: '%s' code: '%s' should be valid: %t", test.data, test.code, test.valid)
	}
}

func testCreateTimeLimitCode(t *testing.T, data string, m int) string {
	result0 := CreateTimeLimitCode(data, m, nil)
	result1 := CreateTimeLimitCode(data, m, time.Now().Format("200601021504"))
	result2 := CreateTimeLimitCode(data, m, time.Unix(time.Now().Unix()+int64(time.Minute)*int64(m), 0).Format("200601021504"))

	assert.Equal(t, result0, result1)
	assert.NotEqual(t, result0, result2)

	assert.True(t, len(result0) != 0)
	return result0
}

func TestFileSize(t *testing.T) {
	var size int64 = 512
	assert.Equal(t, "512 B", FileSize(size))
	size *= 1024
	assert.Equal(t, "512 KiB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512 MiB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512 GiB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512 TiB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512 PiB", FileSize(size))
	size *= 4
	assert.Equal(t, "2.0 EiB", FileSize(size))
}

func TestPrettyNumber(t *testing.T) {
	assert.Equal(t, "23,342,432", PrettyNumber(23342432))
	assert.Equal(t, "0", PrettyNumber(0))
	assert.Equal(t, "-100,000", PrettyNumber(-100000))
}

func TestSubtract(t *testing.T) {
	toFloat64 := func(n interface{}) float64 {
		switch v := n.(type) {
		case int:
			return float64(v)
		case int8:
			return float64(v)
		case int16:
			return float64(v)
		case int32:
			return float64(v)
		case int64:
			return float64(v)
		case float32:
			return float64(v)
		case float64:
			return v
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
	assert.Equal(t, "测...", EllipsisString("测试文本一二三四", 4))
	assert.Equal(t, "测试...", EllipsisString("测试文本一二三四", 5))
	assert.Equal(t, "测试文...", EllipsisString("测试文本一二三四", 6))
	assert.Equal(t, "测试文本一二三四", EllipsisString("测试文本一二三四", 10))
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
	assert.Equal(t, "测试文本", TruncateString("测试文本一二三四", 4))
	assert.Equal(t, "测试文本一", TruncateString("测试文本一二三四", 5))
	assert.Equal(t, "测试文本一二", TruncateString("测试文本一二三四", 6))
	assert.Equal(t, "测试文本一二三", TruncateString("测试文本一二三四", 7))
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

func TestInt64sContains(t *testing.T) {
	assert.Equal(t, map[int64]bool{}, Int64sToMap([]int64{}))
	assert.Equal(t, true, Int64sContains([]int64{6, 44324, 4324, 32, 1, 2323}, 1))
	assert.Equal(t, true, Int64sContains([]int64{2323}, 2323))
	assert.Equal(t, false, Int64sContains([]int64{6, 44324, 4324, 32, 1, 2323}, 232))
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
	assert.False(t, IsLetter(0x00))
	assert.False(t, IsLetter(0x93))
}

func TestDetectContentTypeLongerThanSniffLen(t *testing.T) {
	// Pre-condition: Shorter than sniffLen detects SVG.
	assert.Equal(t, "image/svg+xml", DetectContentType([]byte(`<!-- Comment --><svg></svg>`)))
	// Longer than sniffLen detects something else.
	assert.Equal(t, "text/plain; charset=utf-8", DetectContentType([]byte(`<!--
Comment Comment Comment Comment Comment Comment Comment Comment Comment Comment
Comment Comment Comment Comment Comment Comment Comment Comment Comment Comment
Comment Comment Comment Comment Comment Comment Comment Comment Comment Comment
Comment Comment Comment Comment Comment Comment Comment Comment Comment Comment
Comment Comment Comment Comment Comment Comment Comment Comment Comment Comment
Comment Comment Comment Comment Comment Comment Comment Comment Comment Comment
Comment Comment Comment --><svg></svg>`)))
}

// IsRepresentableAsText

func TestIsTextFile(t *testing.T) {
	assert.True(t, IsTextFile([]byte{}))
	assert.True(t, IsTextFile([]byte("lorem ipsum")))
}

func TestIsImageFile(t *testing.T) {
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAoAAAAKCAYAAACNMs+9AAAAG0lEQVQYlWN4+vTpf3SMDTAMBYXYBLFpHgoKAeiOf0SGE9kbAAAAAElFTkSuQmCC")
	assert.True(t, IsImageFile(png))
	assert.False(t, IsImageFile([]byte("plain text")))
}

func TestIsSVGImageFile(t *testing.T) {
	assert.True(t, IsSVGImageFile([]byte("<svg></svg>")))
	assert.True(t, IsSVGImageFile([]byte("    <svg></svg>")))
	assert.True(t, IsSVGImageFile([]byte(`<svg width="100"></svg>`)))
	assert.True(t, IsSVGImageFile([]byte("<svg/>")))
	assert.True(t, IsSVGImageFile([]byte(`<?xml version="1.0" encoding="UTF-8"?><svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<!-- Comment -->
	<svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<!-- Multiple -->
	<!-- Comments -->
	<svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<!-- Multiline
	Comment -->
	<svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1 Basic//EN"
	"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11-basic.dtd">
	<svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Comment -->
	<svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Multiple -->
	<!-- Comments -->
	<svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Multline
	Comment -->
	<svg></svg>`)))
	assert.True(t, IsSVGImageFile([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
	<!-- Multline
	Comment -->
	<svg></svg>`)))
	assert.False(t, IsSVGImageFile([]byte{}))
	assert.False(t, IsSVGImageFile([]byte("svg")))
	assert.False(t, IsSVGImageFile([]byte("<svgfoo></svgfoo>")))
	assert.False(t, IsSVGImageFile([]byte("text<svg></svg>")))
	assert.False(t, IsSVGImageFile([]byte("<html><body><svg></svg></body></html>")))
	assert.False(t, IsSVGImageFile([]byte(`<script>"<svg></svg>"</script>`)))
	assert.False(t, IsSVGImageFile([]byte(`<!-- <svg></svg> inside comment -->
	<foo></foo>`)))
	assert.False(t, IsSVGImageFile([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- <svg></svg> inside comment -->
	<foo></foo>`)))
}

func TestIsPDFFile(t *testing.T) {
	pdf, _ := base64.StdEncoding.DecodeString("JVBERi0xLjYKJcOkw7zDtsOfCjIgMCBvYmoKPDwvTGVuZ3RoIDMgMCBSL0ZpbHRlci9GbGF0ZURlY29kZT4+CnN0cmVhbQp4nF3NPwsCMQwF8D2f4s2CNYk1baF0EHRwOwg4iJt/NsFb/PpevUE4Mjwe")
	assert.True(t, IsPDFFile(pdf))
	assert.False(t, IsPDFFile([]byte("plain text")))
}

func TestIsVideoFile(t *testing.T) {
	mp4, _ := base64.StdEncoding.DecodeString("AAAAGGZ0eXBtcDQyAAAAAGlzb21tcDQyAAEI721vb3YAAABsbXZoZAAAAADaBlwX2gZcFwAAA+gA")
	assert.True(t, IsVideoFile(mp4))
	assert.False(t, IsVideoFile([]byte("plain text")))
}

func TestIsAudioFile(t *testing.T) {
	mp3, _ := base64.StdEncoding.DecodeString("SUQzBAAAAAABAFRYWFgAAAASAAADbWFqb3JfYnJhbmQAbXA0MgBUWFhYAAAAEQAAA21pbm9yX3Zl")
	assert.True(t, IsAudioFile(mp3))
	assert.False(t, IsAudioFile([]byte("plain text")))
}

// TODO: Test EntryIcon

func TestSetupGiteaRoot(t *testing.T) {
	_ = os.Setenv("GITEA_ROOT", "test")
	assert.EqualValues(t, "test", SetupGiteaRoot())
	_ = os.Setenv("GITEA_ROOT", "")
	assert.NotEqual(t, "test", SetupGiteaRoot())
}

func TestFormatNumberSI(t *testing.T) {
	assert.Equal(t, "125", FormatNumberSI(int(125)))
	assert.Equal(t, "1.3k", FormatNumberSI(int64(1317)))
	assert.Equal(t, "21.3M", FormatNumberSI(21317675))
	assert.Equal(t, "45.7G", FormatNumberSI(45721317675))
	assert.Equal(t, "", FormatNumberSI("test"))
}
