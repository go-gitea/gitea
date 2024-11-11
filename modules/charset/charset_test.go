// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"bytes"
	"io"
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

func TestMaybeRemoveBOM(t *testing.T) {
	res := MaybeRemoveBOM([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, ConvertOpts{})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	res = MaybeRemoveBOM([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, ConvertOpts{})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)
}

func TestToUTF8(t *testing.T) {
	resetDefaultCharsetsOrder()

	// Note: golang compiler seems so behave differently depending on the current
	// locale, so some conversions might behave differently. For that reason, we don't
	// depend on particular conversions but in expected behaviors.

	res, err := ToUTF8([]byte{0x41, 0x42, 0x43}, ConvertOpts{})
	assert.NoError(t, err)
	assert.Equal(t, "ABC", res)

	// "áéíóú"
	res, err = ToUTF8([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, ConvertOpts{})
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, []byte(res))

	// "áéíóú"
	res, err = ToUTF8([]byte{
		0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3,
		0xc3, 0xba,
	}, ConvertOpts{})
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, []byte(res))

	res, err = ToUTF8([]byte{
		0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0xF1, 0x6F, 0x73, 0x41, 0x41, 0x41, 0x2e,
	}, ConvertOpts{})
	assert.NoError(t, err)
	stringMustStartWith(t, "Hola,", res)
	stringMustEndWith(t, "AAA.", res)

	res, err = ToUTF8([]byte{
		0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0x07, 0xA4, 0x6F, 0x73, 0x41, 0x41, 0x41, 0x2e,
	}, ConvertOpts{})
	assert.NoError(t, err)
	stringMustStartWith(t, "Hola,", res)
	stringMustEndWith(t, "AAA.", res)

	res, err = ToUTF8([]byte{
		0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0x81, 0xA4, 0x6F, 0x73, 0x41, 0x41, 0x41, 0x2e,
	}, ConvertOpts{})
	assert.NoError(t, err)
	stringMustStartWith(t, "Hola,", res)
	stringMustEndWith(t, "AAA.", res)

	// Japanese (Shift-JIS)
	// 日属秘ぞしちゅ。
	res, err = ToUTF8([]byte{
		0x93, 0xFA, 0x91, 0xAE, 0x94, 0xE9, 0x82, 0xBC, 0x82, 0xB5, 0x82,
		0xBF, 0x82, 0xE3, 0x81, 0x42,
	}, ConvertOpts{})
	assert.NoError(t, err)
	assert.Equal(t, []byte{
		0xE6, 0x97, 0xA5, 0xE5, 0xB1, 0x9E, 0xE7, 0xA7, 0x98, 0xE3,
		0x81, 0x9E, 0xE3, 0x81, 0x97, 0xE3, 0x81, 0xA1, 0xE3, 0x82, 0x85, 0xE3, 0x80, 0x82,
	},
		[]byte(res))

	res, err = ToUTF8([]byte{0x00, 0x00, 0x00, 0x00}, ConvertOpts{})
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, []byte(res))
}

func TestToUTF8WithFallback(t *testing.T) {
	resetDefaultCharsetsOrder()
	// "ABC"
	res := ToUTF8WithFallback([]byte{0x41, 0x42, 0x43}, ConvertOpts{})
	assert.Equal(t, []byte{0x41, 0x42, 0x43}, res)

	// "áéíóú"
	res = ToUTF8WithFallback([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, ConvertOpts{})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// UTF8 BOM + "áéíóú"
	res = ToUTF8WithFallback([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, ConvertOpts{})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// "Hola, así cómo ños"
	res = ToUTF8WithFallback([]byte{
		0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63,
		0xF3, 0x6D, 0x6F, 0x20, 0xF1, 0x6F, 0x73,
	}, ConvertOpts{})
	assert.Equal(t, []byte{
		0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xC3, 0xAD, 0x20, 0x63,
		0xC3, 0xB3, 0x6D, 0x6F, 0x20, 0xC3, 0xB1, 0x6F, 0x73,
	}, res)

	// "Hola, así cómo "
	minmatch := []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xC3, 0xAD, 0x20, 0x63, 0xC3, 0xB3, 0x6D, 0x6F, 0x20}

	res = ToUTF8WithFallback([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x07, 0xA4, 0x6F, 0x73}, ConvertOpts{})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	res = ToUTF8WithFallback([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x81, 0xA4, 0x6F, 0x73}, ConvertOpts{})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	// Japanese (Shift-JIS)
	// "日属秘ぞしちゅ。"
	res = ToUTF8WithFallback([]byte{0x93, 0xFA, 0x91, 0xAE, 0x94, 0xE9, 0x82, 0xBC, 0x82, 0xB5, 0x82, 0xBF, 0x82, 0xE3, 0x81, 0x42}, ConvertOpts{})
	assert.Equal(t, []byte{
		0xE6, 0x97, 0xA5, 0xE5, 0xB1, 0x9E, 0xE7, 0xA7, 0x98, 0xE3,
		0x81, 0x9E, 0xE3, 0x81, 0x97, 0xE3, 0x81, 0xA1, 0xE3, 0x82, 0x85, 0xE3, 0x80, 0x82,
	}, res)

	res = ToUTF8WithFallback([]byte{0x00, 0x00, 0x00, 0x00}, ConvertOpts{})
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, res)
}

func TestToUTF8DropErrors(t *testing.T) {
	resetDefaultCharsetsOrder()
	// "ABC"
	res := ToUTF8DropErrors([]byte{0x41, 0x42, 0x43}, ConvertOpts{})
	assert.Equal(t, []byte{0x41, 0x42, 0x43}, res)

	// "áéíóú"
	res = ToUTF8DropErrors([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, ConvertOpts{})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// UTF8 BOM + "áéíóú"
	res = ToUTF8DropErrors([]byte{0xef, 0xbb, 0xbf, 0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, ConvertOpts{})
	assert.Equal(t, []byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}, res)

	// "Hola, así cómo ños"
	res = ToUTF8DropErrors([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0xF1, 0x6F, 0x73}, ConvertOpts{})
	assert.Equal(t, []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73}, res[:8])
	assert.Equal(t, []byte{0x73}, res[len(res)-1:])

	// "Hola, así cómo "
	minmatch := []byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xC3, 0xAD, 0x20, 0x63, 0xC3, 0xB3, 0x6D, 0x6F, 0x20}

	res = ToUTF8DropErrors([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x07, 0xA4, 0x6F, 0x73}, ConvertOpts{})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	res = ToUTF8DropErrors([]byte{0x48, 0x6F, 0x6C, 0x61, 0x2C, 0x20, 0x61, 0x73, 0xED, 0x20, 0x63, 0xF3, 0x6D, 0x6F, 0x20, 0x81, 0xA4, 0x6F, 0x73}, ConvertOpts{})
	// Do not fail for differences in invalid cases, as the library might change the conversion criteria for those
	assert.Equal(t, minmatch, res[0:len(minmatch)])

	// Japanese (Shift-JIS)
	// "日属秘ぞしちゅ。"
	res = ToUTF8DropErrors([]byte{0x93, 0xFA, 0x91, 0xAE, 0x94, 0xE9, 0x82, 0xBC, 0x82, 0xB5, 0x82, 0xBF, 0x82, 0xE3, 0x81, 0x42}, ConvertOpts{})
	assert.Equal(t, []byte{
		0xE6, 0x97, 0xA5, 0xE5, 0xB1, 0x9E, 0xE7, 0xA7, 0x98, 0xE3,
		0x81, 0x9E, 0xE3, 0x81, 0x97, 0xE3, 0x81, 0xA1, 0xE3, 0x82, 0x85, 0xE3, 0x80, 0x82,
	}, res)

	res = ToUTF8DropErrors([]byte{0x00, 0x00, 0x00, 0x00}, ConvertOpts{})
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

func stringMustStartWith(t *testing.T, expected, value string) {
	assert.Equal(t, expected, value[:len(expected)])
}

func stringMustEndWith(t *testing.T, expected, value string) {
	assert.Equal(t, expected, value[len(value)-len(expected):])
}

func TestToUTF8WithFallbackReader(t *testing.T) {
	resetDefaultCharsetsOrder()

	for testLen := 0; testLen < 2048; testLen++ {
		pattern := "    test { () }\n"
		input := ""
		for len(input) < testLen {
			input += pattern
		}
		input = input[:testLen]
		input += "// Выключаем"
		rd := ToUTF8WithFallbackReader(bytes.NewReader([]byte(input)), ConvertOpts{})
		r, _ := io.ReadAll(rd)
		assert.EqualValuesf(t, input, string(r), "testing string len=%d", testLen)
	}

	truncatedOneByteExtension := failFastBytes
	encoding, _ := DetectEncoding(truncatedOneByteExtension)
	assert.Equal(t, "UTF-8", encoding)

	truncatedTwoByteExtension := failFastBytes
	truncatedTwoByteExtension[len(failFastBytes)-1] = 0x9b
	truncatedTwoByteExtension[len(failFastBytes)-2] = 0xe2

	encoding, _ = DetectEncoding(truncatedTwoByteExtension)
	assert.Equal(t, "UTF-8", encoding)

	truncatedThreeByteExtension := failFastBytes
	truncatedThreeByteExtension[len(failFastBytes)-1] = 0x92
	truncatedThreeByteExtension[len(failFastBytes)-2] = 0x9f
	truncatedThreeByteExtension[len(failFastBytes)-3] = 0xf0

	encoding, _ = DetectEncoding(truncatedThreeByteExtension)
	assert.Equal(t, "UTF-8", encoding)
}

var failFastBytes = []byte{
	0x69, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x20, 0x6f, 0x72, 0x67, 0x2e, 0x61, 0x70, 0x61, 0x63, 0x68, 0x65, 0x2e, 0x74, 0x6f,
	0x6f, 0x6c, 0x73, 0x2e, 0x61, 0x6e, 0x74, 0x2e, 0x74, 0x61, 0x73, 0x6b, 0x64, 0x65, 0x66, 0x73, 0x2e, 0x63, 0x6f, 0x6e,
	0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x4f, 0x73, 0x0a, 0x69, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x20, 0x6f, 0x72, 0x67,
	0x2e, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x62, 0x6f, 0x6f,
	0x74, 0x2e, 0x67, 0x72, 0x61, 0x64, 0x6c, 0x65, 0x2e, 0x74, 0x61, 0x73, 0x6b, 0x73, 0x2e, 0x72, 0x75, 0x6e, 0x2e, 0x42,
	0x6f, 0x6f, 0x74, 0x52, 0x75, 0x6e, 0x0a, 0x0a, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x20, 0x7b, 0x0a, 0x20, 0x20,
	0x20, 0x20, 0x69, 0x64, 0x28, 0x22, 0x6f, 0x72, 0x67, 0x2e, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d,
	0x65, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x62, 0x6f, 0x6f, 0x74, 0x22, 0x29, 0x0a, 0x7d, 0x0a, 0x0a, 0x64, 0x65, 0x70, 0x65,
	0x6e, 0x64, 0x65, 0x6e, 0x63, 0x69, 0x65, 0x73, 0x20, 0x7b, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65,
	0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x28, 0x22, 0x3a,
	0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x3a, 0x61, 0x70, 0x69, 0x22, 0x29, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d,
	0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74,
	0x28, 0x22, 0x3a, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x3a, 0x61, 0x70, 0x69, 0x2d, 0x64, 0x6f, 0x63, 0x73, 0x22, 0x29,
	0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x28, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x28, 0x22, 0x3a, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x3a, 0x64, 0x62,
	0x22, 0x29, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x28, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x28, 0x22, 0x3a, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x3a,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x22, 0x29, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65,
	0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x28, 0x22, 0x3a,
	0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x3a, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2d, 0x66,
	0x73, 0x22, 0x29, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x28, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x28, 0x22, 0x3a, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72,
	0x3a, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2d, 0x6d, 0x71, 0x22, 0x29, 0x29, 0x0a, 0x0a,
	0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22,
	0x6a, 0x66, 0x75, 0x73, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x65, 0x3a, 0x70, 0x65, 0x2d, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e,
	0x2d, 0x61, 0x75, 0x74, 0x68, 0x2d, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2d, 0x73, 0x74, 0x61, 0x72, 0x74,
	0x65, 0x72, 0x22, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6a, 0x66, 0x75, 0x73, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x65, 0x3a, 0x70, 0x65, 0x2d, 0x63,
	0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2d, 0x68, 0x61, 0x6c, 0x22, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c,
	0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6a, 0x66, 0x75, 0x73, 0x69, 0x6f, 0x6e, 0x2e,
	0x70, 0x65, 0x3a, 0x70, 0x65, 0x2d, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2d, 0x63, 0x6f, 0x72, 0x65, 0x22, 0x29, 0x0a,
	0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28,
	0x22, 0x6f, 0x72, 0x67, 0x2e, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f, 0x72, 0x6b,
	0x2e, 0x62, 0x6f, 0x6f, 0x74, 0x3a, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x2d, 0x62, 0x6f, 0x6f, 0x74, 0x2d, 0x73, 0x74,
	0x61, 0x72, 0x74, 0x65, 0x72, 0x2d, 0x77, 0x65, 0x62, 0x22, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c,
	0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6f, 0x72, 0x67, 0x2e, 0x73, 0x70, 0x72, 0x69,
	0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x62, 0x6f, 0x6f, 0x74, 0x3a, 0x73, 0x70, 0x72,
	0x69, 0x6e, 0x67, 0x2d, 0x62, 0x6f, 0x6f, 0x74, 0x2d, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x72, 0x2d, 0x61, 0x6f, 0x70,
	0x22, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x28, 0x22, 0x6f, 0x72, 0x67, 0x2e, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f,
	0x72, 0x6b, 0x2e, 0x62, 0x6f, 0x6f, 0x74, 0x3a, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x2d, 0x62, 0x6f, 0x6f, 0x74, 0x2d,
	0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x72, 0x2d, 0x61, 0x63, 0x74, 0x75, 0x61, 0x74, 0x6f, 0x72, 0x22, 0x29, 0x0a, 0x20,
	0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6f,
	0x72, 0x67, 0x2e, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x63,
	0x6c, 0x6f, 0x75, 0x64, 0x3a, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x2d, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2d, 0x73, 0x74,
	0x61, 0x72, 0x74, 0x65, 0x72, 0x2d, 0x62, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x22, 0x29, 0x0a, 0x20, 0x20,
	0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6f, 0x72,
	0x67, 0x2e, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x63, 0x6c,
	0x6f, 0x75, 0x64, 0x3a, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x2d, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2d, 0x73, 0x74, 0x61,
	0x72, 0x74, 0x65, 0x72, 0x2d, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2d, 0x61, 0x6c, 0x6c, 0x22, 0x29, 0x0a, 0x20, 0x20,
	0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6f, 0x72,
	0x67, 0x2e, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x63, 0x6c,
	0x6f, 0x75, 0x64, 0x3a, 0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x2d, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2d, 0x73, 0x74, 0x61,
	0x72, 0x74, 0x65, 0x72, 0x2d, 0x73, 0x6c, 0x65, 0x75, 0x74, 0x68, 0x22, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d,
	0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6f, 0x72, 0x67, 0x2e, 0x73, 0x70,
	0x72, 0x69, 0x6e, 0x67, 0x66, 0x72, 0x61, 0x6d, 0x65, 0x77, 0x6f, 0x72, 0x6b, 0x2e, 0x72, 0x65, 0x74, 0x72, 0x79, 0x3a,
	0x73, 0x70, 0x72, 0x69, 0x6e, 0x67, 0x2d, 0x72, 0x65, 0x74, 0x72, 0x79, 0x22, 0x29, 0x0a, 0x0a, 0x20, 0x20, 0x20, 0x20,
	0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x63, 0x68, 0x2e, 0x71,
	0x6f, 0x73, 0x2e, 0x6c, 0x6f, 0x67, 0x62, 0x61, 0x63, 0x6b, 0x3a, 0x6c, 0x6f, 0x67, 0x62, 0x61, 0x63, 0x6b, 0x2d, 0x63,
	0x6c, 0x61, 0x73, 0x73, 0x69, 0x63, 0x22, 0x29, 0x0a, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d,
	0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x69, 0x6f, 0x2e, 0x6d, 0x69, 0x63, 0x72, 0x6f, 0x6d, 0x65,
	0x74, 0x65, 0x72, 0x3a, 0x6d, 0x69, 0x63, 0x72, 0x6f, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x2d, 0x72, 0x65, 0x67, 0x69, 0x73,
	0x74, 0x72, 0x79, 0x2d, 0x70, 0x72, 0x6f, 0x6d, 0x65, 0x74, 0x68, 0x65, 0x75, 0x73, 0x22, 0x29, 0x0a, 0x0a, 0x20, 0x20,
	0x20, 0x20, 0x69, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x6b, 0x6f, 0x74,
	0x6c, 0x69, 0x6e, 0x28, 0x22, 0x73, 0x74, 0x64, 0x6c, 0x69, 0x62, 0x22, 0x29, 0x29, 0x0a, 0x0a, 0x20, 0x20, 0x20, 0x20,
	0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f,
	0x2f, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x2f, 0x2f, 0x20, 0x54, 0x65, 0x73, 0x74, 0x20, 0x64, 0x65, 0x70, 0x65, 0x6e, 0x64,
	0x65, 0x6e, 0x63, 0x69, 0x65, 0x73, 0x2e, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f,
	0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x2f, 0x0a, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x74,
	0x65, 0x73, 0x74, 0x49, 0x6d, 0x70, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x28, 0x22, 0x6a,
	0x66, 0x75, 0x73, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x65, 0x3a, 0x70, 0x65, 0x2d, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2d,
	0x74, 0x65, 0x73, 0x74, 0x22, 0x29, 0x0a, 0x7d, 0x0a, 0x0a, 0x76, 0x61, 0x6c, 0x20, 0x70, 0x61, 0x74, 0x63, 0x68, 0x4a,
	0x61, 0x72, 0x20, 0x62, 0x79, 0x20, 0x74, 0x61, 0x73, 0x6b, 0x73, 0x2e, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72,
	0x69, 0x6e, 0x67, 0x28, 0x4a, 0x61, 0x72, 0x3a, 0x3a, 0x63, 0x6c, 0x61, 0x73, 0x73, 0x29, 0x20, 0x7b, 0x0a, 0x20, 0x20,
	0x20, 0x20, 0x61, 0x72, 0x63, 0x68, 0x69, 0x76, 0x65, 0x43, 0x6c, 0x61, 0x73, 0x73, 0x69, 0x66, 0x69, 0x65, 0x72, 0x2e,
	0x73, 0x65, 0x74, 0x28, 0x22, 0x70, 0x61, 0x74, 0x63, 0x68, 0x65, 0x64, 0x22, 0x29, 0x0a, 0x0a, 0x20, 0x20, 0x20, 0x20,
	0x76, 0x61, 0x6c, 0x20, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x43, 0x6c, 0x61, 0x73, 0x73, 0x70, 0x61, 0x74, 0x68,
	0x20, 0x62, 0x79, 0x20, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x67,
	0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x0a, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x6d, 0x61, 0x6e, 0x69, 0x66, 0x65, 0x73, 0x74,
	0x20, 0x7b, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x61, 0x74, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74, 0x65,
	0x73, 0x28, 0x22, 0x43, 0x6c, 0x61, 0x73, 0x73, 0x2d, 0x50, 0x61, 0x74, 0x68, 0x22, 0x20, 0x74, 0x6f, 0x20, 0x6f, 0x62,
	0x6a, 0x65, 0x63, 0x74, 0x20, 0x7b, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x70,
	0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x20, 0x76, 0x61, 0x6c, 0x20, 0x70, 0x61, 0x74, 0x74, 0x65, 0x72, 0x6e, 0x20, 0x3d,
	0x20, 0x22, 0x66, 0x69, 0x6c, 0x65, 0x3a, 0x2f, 0x2b, 0x22, 0x2e, 0x74, 0x6f, 0x52, 0x65, 0x67, 0x65, 0x78, 0x28, 0x29,
	0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x6f, 0x76, 0x65, 0x72, 0x72, 0x69, 0x64,
	0x65, 0x20, 0x66, 0x75, 0x6e, 0x20, 0x74, 0x6f, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x28, 0x29, 0x3a, 0x20, 0x53, 0x74,
	0x72, 0x69, 0x6e, 0x67, 0x20, 0x3d, 0x20, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x43, 0x6c, 0x61, 0x73, 0x73, 0x70,
	0x61, 0x74, 0x68, 0x2e, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x2e, 0x6a, 0x6f, 0x69, 0x6e, 0x54, 0x6f, 0x53, 0x74, 0x72, 0x69,
	0x6e, 0x67, 0x28, 0x22, 0x20, 0x22, 0x29, 0x20, 0x7b, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x69, 0x74, 0x2e, 0x74, 0x6f, 0x55, 0x52, 0x49, 0x28, 0x29, 0x2e, 0x74, 0x6f, 0x55,
	0x52, 0x4c, 0x28, 0x29, 0x2e, 0x74, 0x6f, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x28, 0x29, 0x2e, 0x72, 0x65, 0x70, 0x6c,
	0x61, 0x63, 0x65, 0x46, 0x69, 0x72, 0x73, 0x74, 0x28, 0x70, 0x61, 0x74, 0x74, 0x65, 0x72, 0x6e, 0x2c, 0x20, 0x22, 0x2f,
	0x22, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x7d, 0x0a, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x7d, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x7d, 0x0a, 0x7d, 0x0a, 0x0a, 0x74, 0x61, 0x73,
	0x6b, 0x73, 0x2e, 0x6e, 0x61, 0x6d, 0x65, 0x64, 0x3c, 0x42, 0x6f, 0x6f, 0x74, 0x52, 0x75, 0x6e, 0x3e, 0x28, 0x22, 0x62,
	0x6f, 0x6f, 0x74, 0x52, 0x75, 0x6e, 0x22, 0x29, 0x20, 0x7b, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x69, 0x66, 0x20, 0x28, 0x4f,
	0x73, 0x2e, 0x69, 0x73, 0x46, 0x61, 0x6d, 0x69, 0x6c, 0x79, 0x28, 0x4f, 0x73, 0x2e, 0x46, 0x41, 0x4d, 0x49, 0x4c, 0x59,
	0x5f, 0x57, 0x49, 0x4e, 0x44, 0x4f, 0x57, 0x53, 0x29, 0x29, 0x20, 0x7b, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x63, 0x6c, 0x61, 0x73, 0x73, 0x70, 0x61, 0x74, 0x68, 0x20, 0x3d, 0x20, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x28, 0x73,
	0x6f, 0x75, 0x72, 0x63, 0x65, 0x53, 0x65, 0x74, 0x73, 0x2e, 0x6e, 0x61, 0x6d, 0x65, 0x64, 0x28, 0x22, 0x6d, 0x61, 0x69,
	0x6e, 0x22, 0x29, 0x2e, 0x6d, 0x61, 0x70, 0x20, 0x7b, 0x20, 0x69, 0x74, 0x2e, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x20,
	0x7d, 0x2c, 0x20, 0x70, 0x61, 0x74, 0x63, 0x68, 0x4a, 0x61, 0x72, 0x29, 0x0a, 0x20, 0x20, 0x20, 0x20, 0x7d, 0x0a, 0x0a,
	0x20, 0x20, 0x20, 0x20, 0x2f, 0x2f, 0x20, 0xd0,
}
