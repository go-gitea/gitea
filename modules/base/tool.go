// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/Unknwon/com"
	"github.com/Unknwon/i18n"
	"github.com/gogits/chardet"
)

// EncodeMD5 encodes string to md5 hex value.
func EncodeMD5(str string) string {
	m := md5.New()
	m.Write([]byte(str))
	return hex.EncodeToString(m.Sum(nil))
}

// EncodeSha1 string to sha1 hex value.
func EncodeSha1(str string) string {
	h := sha1.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// ShortSha is basically just truncating.
// It is DEPRECATED and will be removed in the future.
func ShortSha(sha1 string) string {
	return TruncateString(sha1, 10)
}

// DetectEncoding detect the encoding of content
func DetectEncoding(content []byte) (string, error) {
	if utf8.Valid(content) {
		log.Debug("Detected encoding: utf-8 (fast)")
		return "UTF-8", nil
	}

	result, err := chardet.NewTextDetector().DetectBest(content)
	if err != nil {
		return "", err
	}
	if result.Charset != "UTF-8" && len(setting.Repository.AnsiCharset) > 0 {
		log.Debug("Using default AnsiCharset: %s", setting.Repository.AnsiCharset)
		return setting.Repository.AnsiCharset, err
	}

	log.Debug("Detected encoding: %s", result.Charset)
	return result.Charset, err
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

// GetRandomString generate random string by specify chars.
func GetRandomString(n int) (string, error) {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	buffer := make([]byte, n)
	max := big.NewInt(int64(len(alphanum)))

	for i := 0; i < n; i++ {
		index, err := randomInt(max)
		if err != nil {
			return "", err
		}

		buffer[i] = alphanum[index]
	}

	return string(buffer), nil
}

// GetRandomBytesAsBase64 generates a random base64 string from n bytes
func GetRandomBytesAsBase64(n int) string {
	bytes := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, bytes)

	if err != nil {
		log.Fatal(4, "Error reading random bytes: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(bytes)
}

func randomInt(max *big.Int) (int, error) {
	rand, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}

	return int(rand.Int64()), nil
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
	sh.Write([]byte(data + setting.SecretKey + startStr + endStr + com.ToStr(minutes)))
	encoded := hex.EncodeToString(sh.Sum(nil))

	code := fmt.Sprintf("%s%06d%s", startStr, minutes, encoded)
	return code
}

// HashEmail hashes email address to MD5 string.
// https://en.gravatar.com/site/implement/hash/
func HashEmail(email string) string {
	return EncodeMD5(strings.ToLower(strings.TrimSpace(email)))
}

// AvatarLink returns relative avatar link to the site domain by given email,
// which includes app sub-url as prefix. However, it is possible
// to return full URL if user enables Gravatar-like service.
func AvatarLink(email string) string {
	if setting.EnableFederatedAvatar && setting.LibravatarService != nil {
		// TODO: This doesn't check any error. AvatarLink should return (string, error)
		url, _ := setting.LibravatarService.FromEmail(email)
		return url
	}

	if !setting.DisableGravatar {
		return setting.GravatarSource + HashEmail(email)
	}

	return setting.AppSubURL + "/img/avatar_default.png"
}

// Seconds-based time units
const (
	Minute = 60
	Hour   = 60 * Minute
	Day    = 24 * Hour
	Week   = 7 * Day
	Month  = 30 * Day
	Year   = 12 * Month
)

func computeTimeDiff(diff int64) (int64, string) {
	diffStr := ""
	switch {
	case diff <= 0:
		diff = 0
		diffStr = "now"
	case diff < 2:
		diff = 0
		diffStr = "1 second"
	case diff < 1*Minute:
		diffStr = fmt.Sprintf("%d seconds", diff)
		diff = 0

	case diff < 2*Minute:
		diff -= 1 * Minute
		diffStr = "1 minute"
	case diff < 1*Hour:
		diffStr = fmt.Sprintf("%d minutes", diff/Minute)
		diff -= diff / Minute * Minute

	case diff < 2*Hour:
		diff -= 1 * Hour
		diffStr = "1 hour"
	case diff < 1*Day:
		diffStr = fmt.Sprintf("%d hours", diff/Hour)
		diff -= diff / Hour * Hour

	case diff < 2*Day:
		diff -= 1 * Day
		diffStr = "1 day"
	case diff < 1*Week:
		diffStr = fmt.Sprintf("%d days", diff/Day)
		diff -= diff / Day * Day

	case diff < 2*Week:
		diff -= 1 * Week
		diffStr = "1 week"
	case diff < 1*Month:
		diffStr = fmt.Sprintf("%d weeks", diff/Week)
		diff -= diff / Week * Week

	case diff < 2*Month:
		diff -= 1 * Month
		diffStr = "1 month"
	case diff < 1*Year:
		diffStr = fmt.Sprintf("%d months", diff/Month)
		diff -= diff / Month * Month

	case diff < 2*Year:
		diff -= 1 * Year
		diffStr = "1 year"
	default:
		diffStr = fmt.Sprintf("%d years", diff/Year)
		diff -= (diff / Year) * Year
	}
	return diff, diffStr
}

// MinutesToFriendly returns a user friendly string with number of minutes
// converted to hours and minutes.
func MinutesToFriendly(minutes int) string {
	duration := time.Duration(minutes) * time.Minute
	return TimeSincePro(time.Now().Add(-duration))
}

// TimeSincePro calculates the time interval and generate full user-friendly string.
func TimeSincePro(then time.Time) string {
	return timeSincePro(then, time.Now())
}

func timeSincePro(then, now time.Time) string {
	diff := now.Unix() - then.Unix()

	if then.After(now) {
		return "future"
	}
	if diff == 0 {
		return "now"
	}

	var timeStr, diffStr string
	for {
		if diff == 0 {
			break
		}

		diff, diffStr = computeTimeDiff(diff)
		timeStr += ", " + diffStr
	}
	return strings.TrimPrefix(timeStr, ", ")
}

func timeSince(then, now time.Time, lang string) string {
	lbl := i18n.Tr(lang, "tool.ago")
	diff := now.Unix() - then.Unix()
	if then.After(now) {
		lbl = i18n.Tr(lang, "tool.from_now")
		diff = then.Unix() - now.Unix()
	}

	switch {
	case diff <= 0:
		return i18n.Tr(lang, "tool.now")
	case diff <= 1:
		return i18n.Tr(lang, "tool.1s", lbl)
	case diff < 1*Minute:
		return i18n.Tr(lang, "tool.seconds", diff, lbl)

	case diff < 2*Minute:
		return i18n.Tr(lang, "tool.1m", lbl)
	case diff < 1*Hour:
		return i18n.Tr(lang, "tool.minutes", diff/Minute, lbl)

	case diff < 2*Hour:
		return i18n.Tr(lang, "tool.1h", lbl)
	case diff < 1*Day:
		return i18n.Tr(lang, "tool.hours", diff/Hour, lbl)

	case diff < 2*Day:
		return i18n.Tr(lang, "tool.1d", lbl)
	case diff < 1*Week:
		return i18n.Tr(lang, "tool.days", diff/Day, lbl)

	case diff < 2*Week:
		return i18n.Tr(lang, "tool.1w", lbl)
	case diff < 1*Month:
		return i18n.Tr(lang, "tool.weeks", diff/Week, lbl)

	case diff < 2*Month:
		return i18n.Tr(lang, "tool.1mon", lbl)
	case diff < 1*Year:
		return i18n.Tr(lang, "tool.months", diff/Month, lbl)

	case diff < 2*Year:
		return i18n.Tr(lang, "tool.1y", lbl)
	default:
		return i18n.Tr(lang, "tool.years", diff/Year, lbl)
	}
}

// RawTimeSince retrieves i18n key of time since t
func RawTimeSince(t time.Time, lang string) string {
	return timeSince(t, time.Now(), lang)
}

// TimeSince calculates the time interval and generate user-friendly string.
func TimeSince(then time.Time, lang string) template.HTML {
	return htmlTimeSince(then, time.Now(), lang)
}

func htmlTimeSince(then, now time.Time, lang string) template.HTML {
	return template.HTML(fmt.Sprintf(`<span class="time-since" title="%s">%s</span>`,
		then.Format(setting.TimeFormat),
		timeSince(then, now, lang)))
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

var bytesSizeTable = map[string]uint64{
	"b":  Byte,
	"kb": KByte,
	"mb": MByte,
	"gb": GByte,
	"tb": TByte,
	"pb": PByte,
	"eb": EByte,
}

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
	switch left.(type) {
	case int:
		rleft = int64(left.(int))
	case int8:
		rleft = int64(left.(int8))
	case int16:
		rleft = int64(left.(int16))
	case int32:
		rleft = int64(left.(int32))
	case int64:
		rleft = left.(int64)
	case float32:
		fleft = float64(left.(float32))
		isInt = false
	case float64:
		fleft = left.(float64)
		isInt = false
	}

	switch right.(type) {
	case int:
		rright = int64(right.(int))
	case int8:
		rright = int64(right.(int8))
	case int16:
		rright = int64(right.(int16))
	case int32:
		rright = int64(right.(int32))
	case int64:
		rright = right.(int64)
	case float32:
		fright = float64(right.(float32))
		isInt = false
	case float64:
		fright = right.(float64)
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
	return strings.Index(http.DetectContentType(data), "text/") != -1
}

// IsImageFile detects if data is an image format
func IsImageFile(data []byte) bool {
	return strings.Index(http.DetectContentType(data), "image/") != -1
}

// IsPDFFile detects if data is a pdf format
func IsPDFFile(data []byte) bool {
	return strings.Index(http.DetectContentType(data), "application/pdf") != -1
}

// IsVideoFile detects if data is an video format
func IsVideoFile(data []byte) bool {
	return strings.Index(http.DetectContentType(data), "video/") != -1
}
