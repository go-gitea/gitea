// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"crypto/sha1"
	"fmt"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

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

func TestVerifyTimeLimitCode(t *testing.T) {
	defer test.MockVariableValue(&setting.InstallLock, true)()
	initGeneralSecret := func(secret string) {
		setting.InstallLock = true
		setting.CfgProvider, _ = setting.NewConfigProviderFromData(fmt.Sprintf(`
[oauth2]
JWT_SECRET = %s
`, secret))
		setting.LoadCommonSettings()
	}

	initGeneralSecret("KZb_QLUd4fYVyxetjxC4eZkrBgWM2SndOOWDNtgUUko")
	now := time.Now()

	t.Run("TestGenericParameter", func(t *testing.T) {
		time2000 := time.Date(2000, 1, 2, 3, 4, 5, 0, time.Local)
		assert.Equal(t, "2000010203040000026fa5221b2731b7cf80b1b506f5e39e38c115fee5", CreateTimeLimitCode("test-sha1", 2, time2000, sha1.New()))
		assert.Equal(t, "2000010203040000026fa5221b2731b7cf80b1b506f5e39e38c115fee5", CreateTimeLimitCode("test-sha1", 2, "200001020304", sha1.New()))
		assert.Equal(t, "2000010203040000024842227a2f87041ff82025199c0187410a9297bf", CreateTimeLimitCode("test-hmac", 2, time2000, nil))
		assert.Equal(t, "2000010203040000024842227a2f87041ff82025199c0187410a9297bf", CreateTimeLimitCode("test-hmac", 2, "200001020304", nil))
	})

	t.Run("TestInvalidCode", func(t *testing.T) {
		assert.False(t, VerifyTimeLimitCode(now, "data", 2, ""))
		assert.False(t, VerifyTimeLimitCode(now, "data", 2, "invalid code"))
	})

	t.Run("TestCreateAndVerify", func(t *testing.T) {
		code := CreateTimeLimitCode("data", 2, now, nil)
		assert.False(t, VerifyTimeLimitCode(now.Add(-time.Minute), "data", 2, code)) // not started yet
		assert.True(t, VerifyTimeLimitCode(now, "data", 2, code))
		assert.True(t, VerifyTimeLimitCode(now.Add(time.Minute), "data", 2, code))
		assert.False(t, VerifyTimeLimitCode(now.Add(time.Minute), "DATA", 2, code))   // invalid data
		assert.False(t, VerifyTimeLimitCode(now.Add(2*time.Minute), "data", 2, code)) // expired
	})

	t.Run("TestDifferentSecret", func(t *testing.T) {
		// use another secret to ensure the code is invalid for different secret
		verifyDataCode := func(c string) bool {
			return VerifyTimeLimitCode(now, "data", 2, c)
		}
		code1 := CreateTimeLimitCode("data", 2, now, sha1.New())
		code2 := CreateTimeLimitCode("data", 2, now, nil)
		assert.True(t, verifyDataCode(code1))
		assert.True(t, verifyDataCode(code2))
		initGeneralSecret("000_QLUd4fYVyxetjxC4eZkrBgWM2SndOOWDNtgUUko")
		assert.False(t, verifyDataCode(code1))
		assert.False(t, verifyDataCode(code2))
	})
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
	testSuccess(nil, nil)
	testSuccess([]string{}, []int64{})
	testSuccess([]string{"-1234"}, []int64{-1234})
	testSuccess([]string{"1", "4", "16", "64", "256"}, []int64{1, 4, 16, 64, 256})

	ints, err := StringsToInt64s([]string{"-1", "a"})
	assert.Len(t, ints, 0)
	assert.Error(t, err)
}

func TestInt64sToStrings(t *testing.T) {
	assert.Equal(t, []string{}, Int64sToStrings([]int64{}))
	assert.Equal(t,
		[]string{"1", "4", "16", "64", "256"},
		Int64sToStrings([]int64{1, 4, 16, 64, 256}),
	)
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
