// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/gogs/chardet"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// UTF8BOM is the utf-8 byte-order marker
var UTF8BOM = []byte{'\xef', '\xbb', '\xbf'}

type ConvertOpts struct {
	KeepBOM bool
}

var ToUTF8WithFallbackReaderPrefetchSize = 16 * 1024

// ToUTF8WithFallbackReader detects the encoding of content and converts to UTF-8 reader if possible
func ToUTF8WithFallbackReader(rd io.Reader, opts ConvertOpts) io.Reader {
	buf := make([]byte, ToUTF8WithFallbackReaderPrefetchSize)
	n, err := util.ReadAtMost(rd, buf)
	if err != nil {
		return io.MultiReader(bytes.NewReader(MaybeRemoveBOM(buf[:n], opts)), rd)
	}

	charsetLabel, err := DetectEncoding(buf[:n])
	if err != nil || charsetLabel == "UTF-8" {
		return io.MultiReader(bytes.NewReader(MaybeRemoveBOM(buf[:n], opts)), rd)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		log.Error("Unknown encoding: %s", charsetLabel)
		return io.MultiReader(bytes.NewReader(buf[:n]), rd)
	}

	return transform.NewReader(
		io.MultiReader(
			bytes.NewReader(MaybeRemoveBOM(buf[:n], opts)),
			rd,
		),
		encoding.NewDecoder(),
	)
}

// ToUTF8 converts content to UTF8 encoding
func ToUTF8(content []byte, opts ConvertOpts) ([]byte, error) {
	charsetLabel, err := DetectEncoding(content)
	if err != nil {
		return content, err
	} else if charsetLabel == "UTF-8" {
		return MaybeRemoveBOM(content, opts), nil
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		log.Error("Unknown encoding: %s", charsetLabel)
		return content, fmt.Errorf("unknown encoding: %s", charsetLabel)
	}

	// If there is an error, we concatenate the nicely decoded part and the
	// original left over. This way we won't lose much data.
	result, n, err := transform.Bytes(encoding.NewDecoder(), content)
	if err != nil {
		result = append(result, content[n:]...)
	}

	result = MaybeRemoveBOM(result, opts)

	return result, err
}

// ToUTF8WithFallback detects the encoding of content and converts to UTF-8 if possible
func ToUTF8WithFallback(content []byte, opts ConvertOpts) []byte {
	bs, _ := io.ReadAll(ToUTF8WithFallbackReader(bytes.NewReader(content), opts))
	return bs
}

// ToUTF8DropErrors makes sure the return string is valid utf-8; attempts conversion if possible
func ToUTF8DropErrors(content []byte, opts ConvertOpts) []byte {
	charsetLabel, err := DetectEncoding(content)
	if err != nil || charsetLabel == "UTF-8" {
		return MaybeRemoveBOM(content, opts)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		log.Error("Unknown encoding: %s", charsetLabel)
		return content
	}

	// We ignore any non-decodable parts from the file.
	// Some parts might be lost
	var decoded []byte
	decoder := encoding.NewDecoder()
	idx := 0
	for {
		result, n, err := transform.Bytes(decoder, content[idx:])
		decoded = append(decoded, result...)
		if err == nil {
			break
		}
		decoded = append(decoded, ' ')
		idx = idx + n + 1
		if idx >= len(content) {
			break
		}
	}

	return MaybeRemoveBOM(decoded, opts)
}

// MaybeRemoveBOM removes a UTF-8 BOM from a []byte when opts.KeepBOM is false
func MaybeRemoveBOM(content []byte, opts ConvertOpts) []byte {
	if opts.KeepBOM {
		return content
	}
	if len(content) > 2 && bytes.Equal(content[0:3], UTF8BOM) {
		return content[3:]
	}
	return content
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

	topConfidence := results[0].Confidence
	topResult := results[0]
	priority, has := setting.Repository.DetectedCharsetScore[strings.ToLower(strings.TrimSpace(topResult.Charset))]
	for _, result := range results {
		// As results are sorted in confidence order - if we have a different confidence
		// we know it's less than the current confidence and can break out of the loop early
		if result.Confidence != topConfidence {
			break
		}

		// Otherwise check if this results is earlier in the DetectedCharsetOrder than our current top guess
		resultPriority, resultHas := setting.Repository.DetectedCharsetScore[strings.ToLower(strings.TrimSpace(result.Charset))]
		if resultHas && (!has || resultPriority < priority) {
			topResult = result
			priority = resultPriority
			has = true
		}
	}

	// FIXME: to properly decouple this function the fallback ANSI charset should be passed as an argument
	if topResult.Charset != "UTF-8" && setting.Repository.AnsiCharset != "" {
		return setting.Repository.AnsiCharset, err
	}

	return topResult.Charset, nil
}
