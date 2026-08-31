// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/gogs/chardet"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

var globalVars = sync.OnceValue(func() (ret struct {
	utf8Bom []byte

	defaultWordRegexp   *regexp.Regexp
	ambiguousTableMap   map[string]*AmbiguousTable
	invisibleRangeTable *unicode.RangeTable
},
) {
	ret.utf8Bom = []byte{'\xef', '\xbb', '\xbf'}
	ret.ambiguousTableMap = newAmbiguousTableMap()
	ret.invisibleRangeTable = newInvisibleRangeTable()
	return ret
})

type ConvertOpts struct {
	KeepBOM           bool
	ErrorReplacement  []byte
	ErrorReturnOrigin bool
}

var ToUTF8WithFallbackReaderPrefetchSize = 16 * 1024

// ToUTF8WithFallbackReader detects the encoding of content and converts to UTF-8 reader if possible
func ToUTF8WithFallbackReader(rd io.Reader, opts ConvertOpts) io.Reader {
	buf := make([]byte, ToUTF8WithFallbackReaderPrefetchSize)
	n, err := util.ReadAtMost(rd, buf)
	if err != nil {
		// read error occurs, don't do any processing
		return io.MultiReader(bytes.NewReader(buf[:n]), rd)
	}

	charsetLabel, _ := DetectEncoding(buf[:n])
	if charsetLabel == "UTF-8" {
		// is utf-8, try to remove BOM and read it as-is
		return io.MultiReader(bytes.NewReader(maybeRemoveBOM(buf[:n], opts)), rd)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		// unknown charset, don't do any processing
		return io.MultiReader(bytes.NewReader(buf[:n]), rd)
	}

	// convert from charset to utf-8
	return transform.NewReader(
		io.MultiReader(bytes.NewReader(buf[:n]), rd),
		encoding.NewDecoder(),
	)
}

// ToUTF8WithFallback detects the encoding of content and converts to UTF-8 if possible
func ToUTF8WithFallback(content []byte, opts ConvertOpts) []byte {
	bs, _ := io.ReadAll(ToUTF8WithFallbackReader(bytes.NewReader(content), opts))
	return bs
}

func ToUTF8DropErrors(content []byte) []byte {
	return ToUTF8(content, ConvertOpts{ErrorReplacement: []byte{' '}})
}

func ToUTF8(content []byte, opts ConvertOpts) []byte {
	charsetLabel, _ := DetectEncoding(content)
	if charsetLabel == "UTF-8" {
		return maybeRemoveBOM(content, opts)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		// chardet can return Mozilla-only labels such as IBM424_ltr that html/charset cannot decode
		if opts.ErrorReturnOrigin {
			return content
		}
		return bytes.ToValidUTF8(content, opts.ErrorReplacement)
	}

	var decoded []byte
	decoder := encoding.NewDecoder()
	idx := 0
	for idx < len(content) {
		result, n, err := transform.Bytes(decoder, content[idx:])
		decoded = append(decoded, result...)
		if err == nil {
			break
		}
		if opts.ErrorReturnOrigin {
			return content
		}
		if opts.ErrorReplacement == nil {
			decoded = append(decoded, content[idx+n])
		} else {
			decoded = append(decoded, opts.ErrorReplacement...)
		}
		idx += n + 1
	}
	return maybeRemoveBOM(decoded, opts)
}

// maybeRemoveBOM removes a UTF-8 BOM from a []byte when opts.KeepBOM is false
func maybeRemoveBOM(content []byte, opts ConvertOpts) []byte {
	if opts.KeepBOM {
		return content
	}
	return bytes.TrimPrefix(content, globalVars().utf8Bom)
}

// DetectEncoding detect the encoding of content
// it always returns a detected or guessed "encoding" string, no matter error happens or not
func DetectEncoding(content []byte) (encoding string, _ error) {
	// First we check if the content represents valid utf8 content excepting a truncated character at the end.

	// Now we could decode all the runes in turn but this is not necessarily the cheapest thing to do
	// instead we walk backwards from the end to trim off the incomplete character
	toValidate := content
	end := len(toValidate) - 1

	// U+0000   U+007F 	  0yyyzzzz
	// U+0080   U+07FF 	  110xxxyy 	10yyzzzz
	// U+0800   U+FFFF 	  1110wwww 	10xxxxyy 	10yyzzzz
	// U+010000 U+10FFFF 	11110uvv 	10vvwwww 	10xxxxyy 	10yyzzzz
	cnt := 0
	for end >= 0 && cnt < 4 {
		c := toValidate[end]
		if c>>5 == 0b110 || c>>4 == 0b1110 || c>>3 == 0b11110 {
			// a leading byte
			toValidate = toValidate[:end]
			break
		} else if c>>6 == 0b10 {
			// a continuation byte
			end--
		} else {
			// not an utf-8 byte
			break
		}
		cnt++
	}

	if utf8.Valid(toValidate) {
		return "UTF-8", nil
	}

	textDetector := chardet.NewTextDetector()
	var detectContent []byte
	if len(content) < 1024 {
		// Check if original content is valid
		if _, err := textDetector.DetectBest(content); err != nil {
			return util.IfZero(setting.Repository.AnsiCharset, "UTF-8"), err
		}
		times := 1024 / len(content)
		detectContent = make([]byte, 0, times*len(content))
		for range times {
			detectContent = append(detectContent, content...)
		}
	} else {
		detectContent = content
	}

	// Now we can't use DetectBest or just results[0] because the result isn't stable - so we need a tie-break
	results, err := textDetector.DetectAll(detectContent)
	if err != nil {
		return util.IfZero(setting.Repository.AnsiCharset, "UTF-8"), err
	}

	var (
		topResult     chardet.Result
		topConfidence int
		priority      int
		has           bool
		found         bool
	)
	for _, result := range results {
		if !charsetLabelDecodable(result.Charset) {
			continue
		}
		if !found {
			topResult = result
			topConfidence = result.Confidence
			priority, has = setting.Repository.DetectedCharsetScore[strings.ToLower(strings.TrimSpace(result.Charset))]
			found = true
			continue
		}
		// results are sorted by confidence; once confidence drops, later entries cannot win
		if result.Confidence != topConfidence {
			break
		}
		resultPriority, resultHas := setting.Repository.DetectedCharsetScore[strings.ToLower(strings.TrimSpace(result.Charset))]
		if resultHas && (!has || resultPriority < priority) {
			topResult = result
			priority = resultPriority
			has = true
		}
	}
	if !found {
		return util.IfZero(setting.Repository.AnsiCharset, "UTF-8"), nil
	}

	// FIXME: to properly decouple this function the fallback ANSI charset should be passed as an argument
	if topResult.Charset != "UTF-8" && setting.Repository.AnsiCharset != "" {
		return setting.Repository.AnsiCharset, err
	}

	return topResult.Charset, nil
}

// charsetLabelDecodable reports whether html/charset can decode the label.
// chardet returns Mozilla names such as IBM424_ltr that are not HTML encodings.
func charsetLabelDecodable(label string) bool {
	if strings.EqualFold(strings.TrimSpace(label), "UTF-8") {
		return true
	}
	enc, _ := charset.Lookup(label)
	return enc != nil
}
