// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
)

// EncodeMD5 encodes string to md5 hex value.
func EncodeMD5(str string) string {
	m := md5.New()
	_, _ = m.Write([]byte(str))
	return hex.EncodeToString(m.Sum(nil))
}

// EncodeSha1 string to sha1 hex value.
func EncodeSha1(str string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// EncodeSha256 string to sha1 hex value.
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
	return auth[0], auth[1], nil
}

// BasicAuthEncode encode basic auth string
func BasicAuthEncode(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// GetRandomBytesAsBase64 generates a random base64 string from n bytes
func GetRandomBytesAsBase64(n int) string {
	bytes := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, bytes)

	if err != nil {
		log.Fatal("Error reading random bytes: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(bytes)
}

// VerifyTimeLimitCode verify time limit code
func VerifyTimeLimitCode(data string, minutes int, code string) bool {
	if len(code) <= 18 {
		return false
	}

	// split code
	start := code[:12]
	lives := code[12:18]
	if d, err := com.StrTo(lives).Int(); err == nil {
		minutes = d
	}

	// right active code
	retCode := CreateTimeLimitCode(data, minutes, start)
	if retCode == code && minutes > 0 {
		// check time is expired or not
		before, _ := time.ParseInLocation("200601021504", start, time.Local)
		now := time.Now()
		if before.Add(time.Minute*time.Duration(minutes)).Unix() > now.Unix() {
			return true
		}
	}

	return false
}

// TimeLimitCodeLength default value for time limit code
const TimeLimitCodeLength = 12 + 6 + 40

// CreateTimeLimitCode create a time limit code
// code format: 12 length date time string + 6 minutes string + 40 sha1 encoded string
func CreateTimeLimitCode(data string, minutes int, startInf interface{}) string {
	format := "200601021504"

	var start, end time.Time
	var startStr, endStr string

	if startInf == nil {
		// Use now time create code
		start = time.Now()
		startStr = start.Format(format)
	} else {
		// use start string create code
		startStr = startInf.(string)
		start, _ = time.ParseInLocation(format, startStr, time.Local)
		startStr = start.Format(format)
	}

	end = start.Add(time.Minute * time.Duration(minutes))
	endStr = end.Format(format)

	// create sha1 encode string
	sh := sha1.New()
	_, _ = sh.Write([]byte(data + setting.SecretKey + startStr + endStr + com.ToStr(minutes)))
	encoded := hex.EncodeToString(sh.Sum(nil))

	code := fmt.Sprintf("%s%06d%s", startStr, minutes, encoded)
	return code
}

// HashEmail hashes email address to MD5 string.
// https://en.gravatar.com/site/implement/hash/
func HashEmail(email string) string {
	return EncodeMD5(strings.ToLower(strings.TrimSpace(email)))
}

// DefaultAvatarLink the default avatar link
func DefaultAvatarLink() string {
	return setting.AppSubURL + "/img/avatar_default.png"
}

// DefaultAvatarSize is a sentinel value for the default avatar size, as
// determined by the avatar-hosting service.
const DefaultAvatarSize = -1

// libravatarURL returns the URL for the given email. This function should only
// be called if a federated avatar service is enabled.
func libravatarURL(email string) (*url.URL, error) {
	urlStr, err := setting.LibravatarService.FromEmail(email)
	if err != nil {
		log.Error("LibravatarService.FromEmail(email=%s): error %v", email, err)
		return nil, err
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Error("Failed to parse libravatar url(%s): error %v", urlStr, err)
		return nil, err
	}
	return u, nil
}

// SizedAvatarLink returns a sized link to the avatar for the given email
// address.
func SizedAvatarLink(email string, size int) string {
	var avatarURL *url.URL
	if setting.EnableFederatedAvatar && setting.LibravatarService != nil {
		var err error
		avatarURL, err = libravatarURL(email)
		if err != nil {
			return DefaultAvatarLink()
		}
	} else if !setting.DisableGravatar {
		// copy GravatarSourceURL, because we will modify its Path.
		copyOfGravatarSourceURL := *setting.GravatarSourceURL
		avatarURL = &copyOfGravatarSourceURL
		avatarURL.Path = path.Join(avatarURL.Path, HashEmail(email))
	} else {
		return DefaultAvatarLink()
	}

	vals := avatarURL.Query()
	vals.Set("d", "identicon")
	if size != DefaultAvatarSize {
		vals.Set("s", strconv.Itoa(size))
	}
	avatarURL.RawQuery = vals.Encode()
	return avatarURL.String()
}

// AvatarLink returns relative avatar link to the site domain by given email,
// which includes app sub-url as prefix. However, it is possible
// to return full URL if user enables Gravatar-like service.
func AvatarLink(email string) string {
	return SizedAvatarLink(email, DefaultAvatarSize)
}

// Storage space size types
const (
	Byte  = 1
	KByte = Byte * 1024
	MByte = KByte * 1024
	GByte = MByte * 1024
	TByte = GByte * 1024
	PByte = TByte * 1024
	EByte = PByte * 1024
)

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func humanateBytes(s uint64, base float64, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%dB", s)
	}
	e := math.Floor(logn(float64(s), base))
	suffix := sizes[int(e)]
	val := float64(s) / math.Pow(base, math.Floor(e))
	f := "%.0f"
	if val < 10 {
		f = "%.1f"
	}

	return fmt.Sprintf(f+"%s", val, suffix)
}

// FileSize calculates the file size and generate user-friendly string.
func FileSize(s int64) string {
	sizes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	return humanateBytes(uint64(s), 1024, sizes)
}

// Subtract deals with subtraction of all types of number.
func Subtract(left interface{}, right interface{}) interface{} {
	var rleft, rright int64
	var fleft, fright float64
	var isInt = true
	switch v := left.(type) {
	case int:
		rleft = int64(v)
	case int8:
		rleft = int64(v)
	case int16:
		rleft = int64(v)
	case int32:
		rleft = int64(v)
	case int64:
		rleft = v
	case float32:
		fleft = float64(v)
		isInt = false
	case float64:
		fleft = v
		isInt = false
	}

	switch v := right.(type) {
	case int:
		rright = int64(v)
	case int8:
		rright = int64(v)
	case int16:
		rright = int64(v)
	case int32:
		rright = int64(v)
	case int64:
		rright = v
	case float32:
		fright = float64(v)
		isInt = false
	case float64:
		fright = v
		isInt = false
	}

	if isInt {
		return rleft - rright
	}
	return fleft + float64(rleft) - (fright + float64(rright))
}

// EllipsisString returns a truncated short string,
// it appends '...' in the end of the length of string is too large.
func EllipsisString(str string, length int) string {
	if length <= 3 {
		return "..."
	}
	if len(str) <= length {
		return str
	}
	return str[:length-3] + "..."
}

// TruncateString returns a truncated string with given limit,
// it returns input string if length is not reached limit.
func TruncateString(str string, limit int) string {
	if len(str) < limit {
		return str
	}
	return str[:limit]
}

// StringsToInt64s converts a slice of string to a slice of int64.
func StringsToInt64s(strs []string) ([]int64, error) {
	ints := make([]int64, len(strs))
	for i := range strs {
		n, err := com.StrTo(strs[i]).Int64()
		if err != nil {
			return ints, err
		}
		ints[i] = n
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

// Int64sToMap converts a slice of int64 to a int64 map.
func Int64sToMap(ints []int64) map[int64]bool {
	m := make(map[int64]bool)
	for _, i := range ints {
		m[i] = true
	}
	return m
}

// Int64sContains returns if a int64 in a slice of int64
func Int64sContains(intsSlice []int64, a int64) bool {
	for _, c := range intsSlice {
		if c == a {
			return true
		}
	}
	return false
}

// IsLetter reports whether the rune is a letter (category L).
// https://github.com/golang/go/blob/master/src/go/scanner/scanner.go#L257
func IsLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= 0x80 && unicode.IsLetter(ch)
}

// IsTextFile returns true if file content format is plain text or empty.
func IsTextFile(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	return strings.Contains(http.DetectContentType(data), "text/")
}

// IsImageFile detects if data is an image format
func IsImageFile(data []byte) bool {
	return strings.Contains(http.DetectContentType(data), "image/")
}

// IsPDFFile detects if data is a pdf format
func IsPDFFile(data []byte) bool {
	return strings.Contains(http.DetectContentType(data), "application/pdf")
}

// IsVideoFile detects if data is an video format
func IsVideoFile(data []byte) bool {
	return strings.Contains(http.DetectContentType(data), "video/")
}

// IsAudioFile detects if data is an video format
func IsAudioFile(data []byte) bool {
	return strings.Contains(http.DetectContentType(data), "audio/")
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
			return "file-symlink-directory"
		}
		return "file-symlink-file"
	case entry.IsDir():
		return "file-directory"
	case entry.IsSubModule():
		return "file-submodule"
	}

	return "file-text"
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
