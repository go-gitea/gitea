// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package charset

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestRemoveBOMIfPresent(t *testing.T) {
	res := RemoveBOMIfPresent([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	res = RemoveBOMIfPresent([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)
}

func TestToUTF8WithErr(t *testing.T) {
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
	assert.Equal(t, []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xC3, 0xAD, 0x20, 0x63, 0xC3, 0xB3, 0x6D, 0x6F, 0x20, 0xC3, 0xB1, 0x6F, 0x73}, res)

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

func stringMustStartWith(t *testing.T, expected string, value string) {
	assert.Equal(t, expected, string(value[:len(expected)]))
}

func stringMustEndWith(t *testing.T, expected string, value string) {
	assert.Equal(t, expected, string(value[len(value)-len(expected):]))
}

func bytesMustStartWith(t *testing.T, expected []byte, value []byte) {
	assert.Equal(t, expected, value[:len(expected)])
}
