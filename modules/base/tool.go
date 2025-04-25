// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/dustin/go-humanize"
)

// EncodeSha256 string to sha256 hex value.
func EncodeSha256(str string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// ShortSha is basically just truncating.
// It is DEPRECATED and will be removed in the future.
func ShortSha(sha1 string) string {
	return util.TruncateRunes(sha1, 10)
}

// BasicAuthDecode decode basic auth string
func BasicAuthDecode(encoded string) (string, string, error) {
	s, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}

	if username, password, ok := strings.Cut(string(s), ":"); ok {
		return username, password, nil
	}
	return "", "", errors.New("invalid basic authentication")
}

// VerifyTimeLimitCode verify time limit code
func VerifyTimeLimitCode(now time.Time, data string, minutes int, code string) bool {
	if len(code) <= 18 {
		return false
	}

	startTimeStr := code[:12]
	aliveTimeStr := code[12:18]
	aliveTime, _ := strconv.Atoi(aliveTimeStr) // no need to check err, if anything wrong, the following code check will fail soon

	// check code
	retCode := CreateTimeLimitCode(data, aliveTime, startTimeStr, nil)
	if subtle.ConstantTimeCompare([]byte(retCode), []byte(code)) != 1 {
		return false
	}

	// check time is expired or not: startTime <= now && now < startTime + minutes
	startTime, _ := time.ParseInLocation("200601021504", startTimeStr, time.Local)
	return (startTime.Before(now) || startTime.Equal(now)) && now.Before(startTime.Add(time.Minute*time.Duration(minutes)))
}

// TimeLimitCodeLength default value for time limit code
const TimeLimitCodeLength = 12 + 6 + 40

// CreateTimeLimitCode create a time-limited code.
// Format: 12 length date time string + 6 minutes string (not used) + 40 hash string, some other code depends on this fixed length
// If h is nil, then use the default hmac hash.
func CreateTimeLimitCode[T time.Time | string](data string, minutes int, startTimeGeneric T, h hash.Hash) string {
	const format = "200601021504"

	var start time.Time
	var startTimeAny any = startTimeGeneric
	if t, ok := startTimeAny.(time.Time); ok {
		start = t
	} else {
		var err error
		start, err = time.ParseInLocation(format, startTimeAny.(string), time.Local)
		if err != nil {
			return "" // return an invalid code because the "parse" failed
		}
	}
	startStr := start.Format(format)
	end := start.Add(time.Minute * time.Duration(minutes))

	if h == nil {
		h = hmac.New(sha1.New, setting.GetGeneralTokenSigningSecret())
	}
	_, _ = fmt.Fprintf(h, "%s%s%s%s%d", data, hex.EncodeToString(setting.GetGeneralTokenSigningSecret()), startStr, end.Format(format), minutes)
	encoded := hex.EncodeToString(h.Sum(nil))

	code := fmt.Sprintf("%s%06d%s", startStr, minutes, encoded)
	if len(code) != TimeLimitCodeLength {
		panic("there is a hard requirement for the length of time-limited code") // it shouldn't happen
	}
	return code
}

// FileSize calculates the file size and generate user-friendly string.
func FileSize(s int64) string {
	return humanize.IBytes(uint64(s))
}

// StringsToInt64s converts a slice of string to a slice of int64.
func StringsToInt64s(strs []string) ([]int64, error) {
	if strs == nil {
		return nil, nil
	}
	ints := make([]int64, 0, len(strs))
	for _, s := range strs {
		if s == "" {
			continue
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		ints = append(ints, n)
	}
	return ints, nil
}

// Int64sToStrings converts a slice of int64 to a slice of string.
func Int64sToStrings(ints []int64) []string {
	strs := make([]string, len(ints))
	for i := range ints {
		strs[i] = strconv.FormatInt(ints[i], 10)
	}
	return strs
}
