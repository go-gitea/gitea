// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"crypto/sha1"
	"fmt"
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

	_, _, err = BasicAuthDecode("YWxpY2U=") // "alice", no colon
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
		code := CreateTimeLimitCode("data", 2, now, nil)
		assert.True(t, verifyDataCode(code))
		initGeneralSecret("000_QLUd4fYVyxetjxC4eZkrBgWM2SndOOWDNtgUUko")
		assert.False(t, verifyDataCode(code))
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

func TestStringsToInt64s(t *testing.T) {
	testSuccess := func(input []string, expected []int64) {
		result, err := StringsToInt64s(input)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	}
	testSuccess(nil, nil)
	testSuccess([]string{}, []int64{})
	testSuccess([]string{""}, []int64{})
	testSuccess([]string{"-1234"}, []int64{-1234})
	testSuccess([]string{"1", "4", "16", "64", "256"}, []int64{1, 4, 16, 64, 256})

	ints, err := StringsToInt64s([]string{"-1", "a"})
	assert.Empty(t, ints)
	assert.Error(t, err)
}

func TestInt64sToStrings(t *testing.T) {
	assert.Equal(t, []string{}, Int64sToStrings([]int64{}))
	assert.Equal(t,
		[]string{"1", "4", "16", "64", "256"},
		Int64sToStrings([]int64{1, 4, 16, 64, 256}),
	)
}
