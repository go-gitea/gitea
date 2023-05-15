// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
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

func TestInt64sContains(t *testing.T) {
	assert.True(t, Int64sContains([]int64{6, 44324, 4324, 32, 1, 2323}, 1))
	assert.True(t, Int64sContains([]int64{2323}, 2323))
	assert.False(t, Int64sContains([]int64{6, 44324, 4324, 32, 1, 2323}, 232))
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

// TODO: Test EntryIcon

func TestSetupGiteaRoot(t *testing.T) {
	_ = os.Setenv("GITEA_ROOT", "test")
	assert.Equal(t, "test", SetupGiteaRoot())
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
