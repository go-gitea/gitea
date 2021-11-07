// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package charset

import (
	"reflect"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func resetDefaultCharsetsOrder() {
	defaultDetectedCharsetsOrder := make([]string, 0, len(setting.Repository.DetectedCharsetsOrder))
	for _, charset := range setting.Repository.DetectedCharsetsOrder {
		defaultDetectedCharsetsOrder = append(defaultDetectedCharsetsOrder, strings.ToLower(strings.TrimSpace(charset)))
	}
	setting.Repository.DetectedCharsetScore = map[string]int{}
	i := 0
	for _, charset := range defaultDetectedCharsetsOrder {
		canonicalCharset := strings.ToLower(strings.TrimSpace(charset))
		if _, has := setting.Repository.DetectedCharsetScore[canonicalCharset]; !has {
			setting.Repository.DetectedCharsetScore[canonicalCharset] = i
			i++
		}
	}
}

func TestRemoveBOMIfPresent(t *testing.T) {
	res := RemoveBOMIfPresent([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	res = RemoveBOMIfPresent([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)
}

func TestToUTF8WithErr(t *testing.T) {
	resetDefaultCharsetsOrder()
	var res string
	var err error

	// Note: golang compiler seems so behave differently depending on the current
	// locale, so some conversions might behave differently. For that reason, we don't
	// depend on particular conversions but in expected behaviors.

	res, err = ToUTF8WithErr([]byte{0x41, 0x42, 0x43})
	assert.NoError(t, err)
	assert.Equal(t, "ABC", res)

	// "áéíóú"
	res, err = ToUTF8WithErr([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, []byte(res))

	// "áéíóú"
	res, err = ToUTF8WithErr([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3,
		0xc3, 0xba})
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, []byte(res))

	res, err = ToUTF8WithErr([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0xF1, 0x6F, 0x73, 0x41, 0x41, 0x41, 0x2e})
	assert.NoError(t, err)
	stringMustStartWith(t, "Hola,", res)
	stringMustEndWith(t, "AAA.", res)

	res, err = ToUTF8WithErr([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0x07, 0xA4, 0x6F, 0x73, 0x41, 0x41, 0x41, 0x2e})
	assert.NoError(t, err)
	stringMustStartWith(t, "Hola,", res)
	stringMustEndWith(t, "AAA.", res)

	res, err = ToUTF8WithErr([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0x81, 0xA4, 0x6F, 0x73, 0x41, 0x41, 0x41, 0x2e})
	assert.NoError(t, err)
	stringMustStartWith(t, "Hola,", res)
	stringMustEndWith(t, "AAA.", res)

	// Japanese (Shift-JIS)
	// 日属秘ぞしちゅ。
	res, err = ToUTF8WithErr([]byte{0x93, 0xFA, 0x91, 0xAE, 0x94, 0xE9, 0x82, 0xBC, 0x82, 0xB5, 0x82,
		0xBF, 0x82, 0xE3, 0x81, 0x42})
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xE6, 0x97, 0xA5, 0xE5, 0xB1, 0x9E, 0xE7, 0xA7, 0x98, 0xE3,
		0x81, 0x9E, 0xE3, 0x81, 0x97, 0xE3, 0x81, 0xA1, 0xE3, 0x82, 0x85, 0xE3, 0x80, 0x82},
		[]byte(res))

	res, err = ToUTF8WithErr([]byte{0x00, 0x00, 0x00, 0x00})
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, []byte(res))
}

func TestToUTF8WithFallback(t *testing.T) {
	resetDefaultCharsetsOrder()
	// "ABC"
	res := ToUTF8WithFallback([]byte{0x41, 0x42, 0x43})
	assert.Equal(t, []byte{0x41, 0x42, 0x43}, res)

	// "áéíóú"
	res = ToUTF8WithFallback([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// UTF8 BOM + "áéíóú"
	res = ToUTF8WithFallback([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// "Hola, así cómo ños"
	res = ToUTF8WithFallback([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0xF1, 0x6F, 0x73})
	assert.Equal(t, []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xC3, 0xAD, 0x20, 0x63,
		0xC3, 0xB3, 0x6D, 0x6F, 0x20, 0xC3, 0xB1, 0x6F, 0x73}, res)

	// "Hola, así cómo "
	minmatch := []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xC3, 0xAD, 0x20, 0x63, 0xC3, 0xB3, 0x6D, 0x6F, 0x20}

	res = ToUTF8WithFallback([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x07, 0xA4, 0x6F, 0x73})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	res = ToUTF8WithFallback([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x81, 0xA4, 0x6F, 0x73})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	// Japanese (Shift-JIS)
	// "日属秘ぞしちゅ。"
	res = ToUTF8WithFallback([]byte{0x93, 0xFA, 0x91, 0xAE, 0x94, 0xE9, 0x82, 0xBC, 0x82, 0xB5, 0x82, 0xBF, 0x82, 0xE3, 0x81, 0x42})
	assert.Equal(t, []byte{0xE6, 0x97, 0xA5, 0xE5, 0xB1, 0x9E, 0xE7, 0xA7, 0x98, 0xE3,
		0x81, 0x9E, 0xE3, 0x81, 0x97, 0xE3, 0x81, 0xA1, 0xE3, 0x82, 0x85, 0xE3, 0x80, 0x82}, res)

	res = ToUTF8WithFallback([]byte{0x00, 0x00, 0x00, 0x00})
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, res)
}

func TestToUTF8(t *testing.T) {
	resetDefaultCharsetsOrder()
	// Note: golang compiler seems so behave differently depending on the current
	// locale, so some conversions might behave differently. For that reason, we don't
	// depend on particular conversions but in expected behaviors.

	res := ToUTF8(string([]byte{0x41, 0x42, 0x43}))
	assert.Equal(t, "ABC", res)

	// "áéíóú"
	res = ToUTF8(string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}))
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, []byte(res))

	// BOM + "áéíóú"
	res = ToUTF8(string([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3,
		0xc3, 0xba}))
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, []byte(res))

	// Latin1
	// Hola, así cómo ños
	res = ToUTF8(string([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0xF1, 0x6F, 0x73}))
	assert.Equal(t, []byte{0x48, 0x6f, 0x6c, 0x61, 0x2c, 0x20, 0x61, 0x73, 0xc3, 0xad, 0x20, 0x63,
		0xc3, 0xb3, 0x6d, 0x6f, 0x20, 0xc3, 0xb1, 0x6f, 0x73}, []byte(res))

	// Latin1
	// Hola, así cómo \x07ños
	res = ToUTF8(string([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0x07, 0xA4, 0x6F, 0x73}))
	// Hola,
	bytesMustStartWith(t, []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C}, []byte(res))

	// This test FAILS
	// res = ToUTF8("Hola, así cómo \x81ños")
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	// assert.Regexp(t, "^Hola, así cómo", res)

	// Japanese (Shift-JIS)
	// 日属秘ぞしちゅ。
	res = ToUTF8(string([]byte{0x93, 0xFA, 0x91, 0xAE, 0x94, 0xE9, 0x82, 0xBC, 0x82, 0xB5, 0x82,
		0xBF, 0x82, 0xE3, 0x81, 0x42}))
	assert.Equal(t, []byte{0xE6, 0x97, 0xA5, 0xE5, 0xB1, 0x9E, 0xE7, 0xA7, 0x98, 0xE3,
		0x81, 0x9E, 0xE3, 0x81, 0x97, 0xE3, 0x81, 0xA1, 0xE3, 0x82, 0x85, 0xE3, 0x80, 0x82},
		[]byte(res))

	res = ToUTF8("\x00\x00\x00\x00")
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, []byte(res))
}

func TestToUTF8DropErrors(t *testing.T) {
	resetDefaultCharsetsOrder()
	// "ABC"
	res := ToUTF8DropErrors([]byte{0x41, 0x42, 0x43})
	assert.Equal(t, []byte{0x41, 0x42, 0x43}, res)

	// "áéíóú"
	res = ToUTF8DropErrors([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// UTF8 BOM + "áéíóú"
	res = ToUTF8DropErrors([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// "Hola, así cómo ños"
	res = ToUTF8DropErrors([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0xF1, 0x6F, 0x73})
	assert.Equal(t, []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73}, res[:8])
	assert.Equal(t, []byte{0x73}, res[len(res)-1:])

	// "Hola, así cómo "
	minmatch := []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xC3, 0xAD, 0x20, 0x63, 0xC3, 0xB3, 0x6D, 0x6F, 0x20}

	res = ToUTF8DropErrors([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x07, 0xA4, 0x6F, 0x73})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	res = ToUTF8DropErrors([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x81, 0xA4, 0x6F, 0x73})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	// Japanese (Shift-JIS)
	// "日属秘ぞしちゅ。"
	res = ToUTF8DropErrors([]byte{0x93, 0xFA, 0x91, 0xAE, 0x94, 0xE9, 0x82, 0xBC, 0x82, 0xB5, 0x82, 0xBF, 0x82, 0xE3, 0x81, 0x42})
	assert.Equal(t, []byte{0xE6, 0x97, 0xA5, 0xE5, 0xB1, 0x9E, 0xE7, 0xA7, 0x98, 0xE3,
		0x81, 0x9E, 0xE3, 0x81, 0x97, 0xE3, 0x81, 0xA1, 0xE3, 0x82, 0x85, 0xE3, 0x80, 0x82}, res)

	res = ToUTF8DropErrors([]byte{0x00, 0x00, 0x00, 0x00})
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, res)
}

func TestDetectEncoding(t *testing.T) {
	resetDefaultCharsetsOrder()
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
	assert.Contains(t, encoding, "ISO-8859-1")

	old := setting.Repository.AnsiCharset
	setting.Repository.AnsiCharset = "placeholder"
	defer func() {
		setting.Repository.AnsiCharset = old
	}()
	testSuccess(b, "placeholder")

	// invalid bytes
	b = []byte{0xfa}
	_, err = DetectEncoding(b)
	assert.Error(t, err)
}

func stringMustStartWith(t *testing.T, expected string, value string) {
	assert.Equal(t, expected, string(value[:len(expected)]))
}

func stringMustEndWith(t *testing.T, expected string, value string) {
	assert.Equal(t, expected, string(value[len(value)-len(expected):]))
}

func bytesMustStartWith(t *testing.T, expected []byte, value []byte) {
	assert.Equal(t, expected, value[:len(expected)])
}

type escapeControlTest struct {
	name   string
	text   string
	status EscapeStatus
	result string
}

var escapeControlTests = []escapeControlTest{
	{
		name: "<empty>",
	},
	{
		name:   "single line western",
		text:   "single line western",
		result: "single line western",
		status: EscapeStatus{HasLTRScript: true},
	},
	{
		name:   "multi line western",
		text:   "single line western\nmulti line western\n",
		result: "single line western\nmulti line western\n",
		status: EscapeStatus{HasLTRScript: true},
	},
	{
		name:   "multi line western non-breaking space",
		text:   "single line western\nmulti line western\n",
		result: `single line<span class="escaped-code-point" escaped="[U+00A0]"><span class="char"> </span></span>western` + "\n" + `multi line<span class="escaped-code-point" escaped="[U+00A0]"><span class="char"> </span></span>western` + "\n",
		status: EscapeStatus{Escaped: true, HasLTRScript: true, HasSpaces: true},
	},
	{
		name:   "mixed scripts: western + japanese",
		text:   "日属秘ぞしちゅ。Then some western.",
		result: "日属秘ぞしちゅ。Then some western.",
		status: EscapeStatus{HasLTRScript: true},
	},
	{
		name:   "japanese",
		text:   "日属秘ぞしちゅ。",
		result: "日属秘ぞしちゅ。",
		status: EscapeStatus{HasLTRScript: true},
	},
	{
		name:   "hebrew",
		text:   "עד תקופת יוון העתיקה היה העיסוק במתמטיקה תכליתי בלבד: היא שימשה כאוסף של נוסחאות לחישוב קרקע, אוכלוסין וכו'. פריצת הדרך של היוונים, פרט לתרומותיהם הגדולות לידע המתמטי, הייתה בלימוד המתמטיקה כשלעצמה, מתוקף ערכה הרוחני. יחסם של חלק מהיוונים הקדמונים למתמטיקה היה דתי - למשל, הכת שאסף סביבו פיתגורס האמינה כי המתמטיקה היא הבסיס לכל הדברים. היוונים נחשבים ליוצרי מושג ההוכחה המתמטית, וכן לראשונים שעסקו במתמטיקה לשם עצמה, כלומר כתחום מחקרי עיוני ומופשט ולא רק כעזר שימושי. עם זאת, לצדה",
		result: "עד תקופת יוון העתיקה היה העיסוק במתמטיקה תכליתי בלבד: היא שימשה כאוסף של נוסחאות לחישוב קרקע, אוכלוסין וכו'. פריצת הדרך של היוונים, פרט לתרומותיהם הגדולות לידע המתמטי, הייתה בלימוד המתמטיקה כשלעצמה, מתוקף ערכה הרוחני. יחסם של חלק מהיוונים הקדמונים למתמטיקה היה דתי - למשל, הכת שאסף סביבו פיתגורס האמינה כי המתמטיקה היא הבסיס לכל הדברים. היוונים נחשבים ליוצרי מושג ההוכחה המתמטית, וכן לראשונים שעסקו במתמטיקה לשם עצמה, כלומר כתחום מחקרי עיוני ומופשט ולא רק כעזר שימושי. עם זאת, לצדה",
		status: EscapeStatus{HasRTLScript: true},
	},
	{
		name: "more hebrew",
		text: `בתקופה מאוחרת יותר, השתמשו היוונים בשיטת סימון מתקדמת יותר, שבה הוצגו המספרים לפי 22 אותיות האלפבית היווני. לסימון המספרים בין 1 ל-9 נקבעו תשע האותיות הראשונות, בתוספת גרש ( ' ) בצד ימין של האות, למעלה; תשע האותיות הבאות ייצגו את העשרות מ-10 עד 90, והבאות את המאות. לסימון הספרות בין 1000 ל-900,000, השתמשו היוונים באותן אותיות, אך הוסיפו לאותיות את הגרש דווקא מצד שמאל של האותיות, למטה. ממיליון ומעלה, כנראה השתמשו היוונים בשני תגים במקום אחד.

			המתמטיקאי הבולט הראשון ביוון העתיקה, ויש האומרים בתולדות האנושות, הוא תאלס (624 לפנה"ס - 546 לפנה"ס בקירוב).[1] לא יהיה זה משולל יסוד להניח שהוא האדם הראשון שהוכיח משפט מתמטי, ולא רק גילה אותו. תאלס הוכיח שישרים מקבילים חותכים מצד אחד של שוקי זווית קטעים בעלי יחסים שווים (משפט תאלס הראשון), שהזווית המונחת על קוטר במעגל היא זווית ישרה (משפט תאלס השני), שהקוטר מחלק את המעגל לשני חלקים שווים, ושזוויות הבסיס במשולש שווה-שוקיים שוות זו לזו. מיוחסות לו גם שיטות למדידת גובהן של הפירמידות בעזרת מדידת צילן ולקביעת מיקומה של ספינה הנראית מן החוף.

			בשנים 582 לפנה"ס עד 496 לפנה"ס, בקירוב, חי מתמטיקאי חשוב במיוחד - פיתגורס. המקורות הראשוניים עליו מועטים, וההיסטוריונים מתקשים להפריד את העובדות משכבת המסתורין והאגדות שנקשרו בו. ידוע שסביבו התקבצה האסכולה הפיתגוראית מעין כת פסבדו-מתמטית שהאמינה ש"הכל מספר", או ליתר דיוק הכל ניתן לכימות, וייחסה למספרים משמעויות מיסטיות. ככל הנראה הפיתגוראים ידעו לבנות את הגופים האפלטוניים, הכירו את הממוצע האריתמטי, הממוצע הגאומטרי והממוצע ההרמוני והגיעו להישגים חשובים נוספים. ניתן לומר שהפיתגוראים גילו את היותו של השורש הריבועי של 2, שהוא גם האלכסון בריבוע שאורך צלעותיו 1, אי רציונלי, אך תגליתם הייתה למעשה רק שהקטעים "חסרי מידה משותפת", ומושג המספר האי רציונלי מאוחר יותר.[2] אזכור ראשון לקיומם של קטעים חסרי מידה משותפת מופיע בדיאלוג "תאיטיטוס" של אפלטון, אך רעיון זה היה מוכר עוד קודם לכן, במאה החמישית לפנה"ס להיפאסוס, בן האסכולה הפיתגוראית, ואולי לפיתגורס עצמו.[3]`,
		result: `בתקופה מאוחרת יותר, השתמשו היוונים בשיטת סימון מתקדמת יותר, שבה הוצגו המספרים לפי 22 אותיות האלפבית היווני. לסימון המספרים בין 1 ל-9 נקבעו תשע האותיות הראשונות, בתוספת גרש ( ' ) בצד ימין של האות, למעלה; תשע האותיות הבאות ייצגו את העשרות מ-10 עד 90, והבאות את המאות. לסימון הספרות בין 1000 ל-900,000, השתמשו היוונים באותן אותיות, אך הוסיפו לאותיות את הגרש דווקא מצד שמאל של האותיות, למטה. ממיליון ומעלה, כנראה השתמשו היוונים בשני תגים במקום אחד.

			המתמטיקאי הבולט הראשון ביוון העתיקה, ויש האומרים בתולדות האנושות, הוא תאלס (624 לפנה"ס - 546 לפנה"ס בקירוב).[1] לא יהיה זה משולל יסוד להניח שהוא האדם הראשון שהוכיח משפט מתמטי, ולא רק גילה אותו. תאלס הוכיח שישרים מקבילים חותכים מצד אחד של שוקי זווית קטעים בעלי יחסים שווים (משפט תאלס הראשון), שהזווית המונחת על קוטר במעגל היא זווית ישרה (משפט תאלס השני), שהקוטר מחלק את המעגל לשני חלקים שווים, ושזוויות הבסיס במשולש שווה-שוקיים שוות זו לזו. מיוחסות לו גם שיטות למדידת גובהן של הפירמידות בעזרת מדידת צילן ולקביעת מיקומה של ספינה הנראית מן החוף.

			בשנים 582 לפנה"ס עד 496 לפנה"ס, בקירוב, חי מתמטיקאי חשוב במיוחד - פיתגורס. המקורות הראשוניים עליו מועטים, וההיסטוריונים מתקשים להפריד את העובדות משכבת המסתורין והאגדות שנקשרו בו. ידוע שסביבו התקבצה האסכולה הפיתגוראית מעין כת פסבדו-מתמטית שהאמינה ש"הכל מספר", או ליתר דיוק הכל ניתן לכימות, וייחסה למספרים משמעויות מיסטיות. ככל הנראה הפיתגוראים ידעו לבנות את הגופים האפלטוניים, הכירו את הממוצע האריתמטי, הממוצע הגאומטרי והממוצע ההרמוני והגיעו להישגים חשובים נוספים. ניתן לומר שהפיתגוראים גילו את היותו של השורש הריבועי של 2, שהוא גם האלכסון בריבוע שאורך צלעותיו 1, אי רציונלי, אך תגליתם הייתה למעשה רק שהקטעים "חסרי מידה משותפת", ומושג המספר האי רציונלי מאוחר יותר.[2] אזכור ראשון לקיומם של קטעים חסרי מידה משותפת מופיע בדיאלוג "תאיטיטוס" של אפלטון, אך רעיון זה היה מוכר עוד קודם לכן, במאה החמישית לפנה"ס להיפאסוס, בן האסכולה הפיתגוראית, ואולי לפיתגורס עצמו.[3]`,
		status: EscapeStatus{HasRTLScript: true},
	},
	{
		name: "Mixed RTL+LTR",
		text: `Many computer programs fail to display bidirectional text correctly.
For example, the Hebrew name Sarah (שרה) is spelled: sin (ש) (which appears rightmost),
then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		result: `Many computer programs fail to display bidirectional text correctly.
For example, the Hebrew name Sarah (שרה) is spelled: sin (ש) (which appears rightmost),
then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		status: EscapeStatus{
			HasRTLScript: true,
			HasLTRScript: true,
		},
	},
	{
		name: "Mixed RTL+LTR+BIDI",
		text: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah ` + "\u2067" + `שרה` + "\u2066\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		result: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah <span class="escaped-code-point" escaped="[U+2067]"><span class="char">` + "\u2067" + `</span></span>שרה<span class="escaped-code-point" escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>` + "\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		status: EscapeStatus{
			Escaped:      true,
			HasBIDI:      true,
			HasRTLScript: true,
			HasLTRScript: true,
		},
	},
	{
		name:   "Accented characters",
		text:   string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}),
		result: string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}),
		status: EscapeStatus{HasLTRScript: true},
	},
	{
		name:   "Program",
		text:   "string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})",
		result: "string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})",
		status: EscapeStatus{HasLTRScript: true},
	},
	{
		name:   "CVE testcase",
		text:   "if access_level != \"user\u202E \u2066// Check if admin\u2069 \u2066\" {",
		result: `if access_level != "user<span class="escaped-code-point" escaped="[U+202E]"><span class="char">` + "\u202e" + `</span></span> <span class="escaped-code-point" escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>// Check if admin<span class="escaped-code-point" escaped="[U+2069]"><span class="char">` + "\u2069" + `</span></span> <span class="escaped-code-point" escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>" {`,
		status: EscapeStatus{Escaped: true, HasBIDI: true, BadBIDI: true, HasLTRScript: true},
	},
	{
		name: "Mixed testcase with fail",
		text: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah ` + "\u2067" + `שרה` + "\u2066\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).` +
			"\nif access_level != \"user\u202E \u2066// Check if admin\u2069 \u2066\" {\n",
		result: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah <span class="escaped-code-point" escaped="[U+2067]"><span class="char">` + "\u2067" + `</span></span>שרה<span class="escaped-code-point" escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>` + "\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).` +
			"\n" + `if access_level != "user<span class="escaped-code-point" escaped="[U+202E]"><span class="char">` + "\u202e" + `</span></span> <span class="escaped-code-point" escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>// Check if admin<span class="escaped-code-point" escaped="[U+2069]"><span class="char">` + "\u2069" + `</span></span> <span class="escaped-code-point" escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>" {` + "\n",
		status: EscapeStatus{Escaped: true, HasBIDI: true, BadBIDI: true, HasLTRScript: true, HasRTLScript: true},
	},
}

func TestEscapeControlString(t *testing.T) {
	for _, tt := range escapeControlTests {
		t.Run(tt.name, func(t *testing.T) {
			status, result := EscapeControlString(tt.text)
			if !reflect.DeepEqual(status, tt.status) {
				t.Errorf("EscapeControlString() status = %v, wanted= %v", status, tt.status)
			}
			if result != tt.result {
				t.Errorf("EscapeControlString()\nresult= %v,\nwanted= %v", result, tt.result)
			}
		})
	}
}

func TestEscapeControlBytes(t *testing.T) {
	for _, tt := range escapeControlTests {
		t.Run(tt.name, func(t *testing.T) {
			status, result := EscapeControlBytes([]byte(tt.text))
			if !reflect.DeepEqual(status, tt.status) {
				t.Errorf("EscapeControlBytes() status = %v, wanted= %v", status, tt.status)
			}
			if string(result) != tt.result {
				t.Errorf("EscapeControlBytes()\nresult= %v,\nwanted= %v", result, tt.result)
			}
		})
	}
}

func TestEscapeControlReader(t *testing.T) {
	// lets add some control characters to the tests
	tests := make([]escapeControlTest, 0, len(escapeControlTests)*3)
	copy(tests, escapeControlTests)
	for _, test := range escapeControlTests {
		test.name += " (+Control)"
		test.text = "\u001E" + test.text
		test.result = `<span class="escaped-code-point" escaped="[U+001E]"><span class="char">` + "\u001e" + `</span></span>` + test.result
		test.status.Escaped = true
		test.status.HasControls = true
		tests = append(tests, test)
	}

	for _, test := range escapeControlTests {
		test.name += " (+Mark)"
		test.text = "\u0300" + test.text
		test.result = `<span class="escaped-code-point" escaped="[U+0300]"><span class="char">` + "\u0300" + `</span></span>` + test.result
		test.status.Escaped = true
		test.status.HasMarks = true
		tests = append(tests, test)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.text)
			output := &strings.Builder{}
			status, err := EscapeControlReader(input, output)
			result := output.String()
			if err != nil {
				t.Errorf("EscapeControlReader(): err = %v", err)
			}

			if !reflect.DeepEqual(status, tt.status) {
				t.Errorf("EscapeControlReader() status = %v, wanted= %v", status, tt.status)
			}
			if result != tt.result {
				t.Errorf("EscapeControlReader()\nresult= %v,\nwanted= %v", result, tt.result)
			}
		})
	}
}
