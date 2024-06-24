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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

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
	return TruncateString(sha1, 10)
}

// BasicAuthDecode decode basic auth string
func BasicAuthDecode(encoded string) (string, string, error) {
	s, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}

	auth := strings.SplitN(string(s), ":", 2)

	if len(auth) != 2 {
		return "", "", errors.New("invalid basic authentication")
	}

	return auth[0], auth[1], nil
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
		retCode = CreateTimeLimitCode(data, aliveTime, startTimeStr, sha1.New()) // TODO: this is only for the support of legacy codes, remove this in/after 1.23
		if subtle.ConstantTimeCompare([]byte(retCode), []byte(code)) != 1 {
			return false
		}
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

// EllipsisString returns a truncated short string,
// it appends '...' in the end of the length of string is too large.
func EllipsisString(str string, length int) string {
	if length <= 3 {
		return "..."
	}
	if utf8.RuneCountInString(str) <= length {
		return str
	}
	return string([]rune(str)[:length-3]) + "..."
}

// TruncateString returns a truncated string with given limit,
// it returns input string if length is not reached limit.
func TruncateString(str string, limit int) string {
	if utf8.RuneCountInString(str) < limit {
		return str
	}
	return string([]rune(str)[:limit])
}

// StringsToInt64s converts a slice of string to a slice of int64.
func StringsToInt64s(strs []string) ([]int64, error) {
	if strs == nil {
		return nil, nil
	}
	ints := make([]int64, 0, len(strs))
	for _, s := range strs {
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

// EntryIcon returns the octicon class for displaying files/directories
func EntryIcon(entry *git.TreeEntry) string {
	switch {
	case entry.IsLink():
		te, err := entry.FollowLink()
		if err != nil {
			log.Debug(err.Error())
			return "file-symlink-file"
		}
		if te.IsDir() {
			return "file-directory-symlink"
		}
		return "file-symlink-file"
	case entry.IsDir():
		return "file-directory-fill"
	case entry.IsSubModule():
		return "file-submodule"
	}

	return "file"
}

// SetupGiteaRoot Sets GITEA_ROOT if it is not already set and returns the value
func SetupGiteaRoot() string {
	giteaRoot := os.Getenv("GITEA_ROOT")
	if giteaRoot == "" {
		_, filename, _, _ := runtime.Caller(0)
		giteaRoot = strings.TrimSuffix(filename, "modules/base/tool.go")
		wd, err := os.Getwd()
		if err != nil {
			rel, err := filepath.Rel(giteaRoot, wd)
			if err != nil && strings.HasPrefix(filepath.ToSlash(rel), "../") {
				giteaRoot = wd
			}
		}
		if _, err := os.Stat(filepath.Join(giteaRoot, "gitea")); os.IsNotExist(err) {
			giteaRoot = ""
		} else if err := os.Setenv("GITEA_ROOT", giteaRoot); err != nil {
			giteaRoot = ""
		}
	}
	return giteaRoot
}

// FormatNumberSI format a number
func FormatNumberSI(data any) string {
	var num int64
	if num1, ok := data.(int64); ok {
		num = num1
	} else if num1, ok := data.(int); ok {
		num = int64(num1)
	} else {
		return ""
	}

	if num < 1000 {
		return fmt.Sprintf("%d", num)
	} else if num < 1000000 {
		num2 := float32(num) / float32(1000.0)
		return fmt.Sprintf("%.1fk", num2)
	} else if num < 1000000000 {
		num2 := float32(num) / float32(1000000.0)
		return fmt.Sprintf("%.1fM", num2)
	}
	num2 := float32(num) / float32(1000000000.0)
	return fmt.Sprintf("%.1fG", num2)
}
