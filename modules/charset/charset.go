// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package charset

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/gogs/chardet"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// UTF8BOM is the utf-8 byte-order marker
var UTF8BOM = []byte{'\xef', '\xbb', '\xbf'}

// ToUTF8WithFallbackReader detects the encoding of content and coverts to UTF-8 reader if possible
func ToUTF8WithFallbackReader(rd io.Reader) io.Reader {
	var buf = make([]byte, 2048)
	n, err := rd.Read(buf)
	if err != nil {
		return rd
	}

	charsetLabel, err := DetectEncoding(buf[:n])
	if err != nil || charsetLabel == "UTF-8" {
		return io.MultiReader(bytes.NewReader(RemoveBOMIfPresent(buf[:n])), rd)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return io.MultiReader(bytes.NewReader(buf[:n]), rd)
	}

	return transform.NewReader(
		io.MultiReader(
			bytes.NewReader(RemoveBOMIfPresent(buf[:n])),
			rd,
		),
		encoding.NewDecoder(),
	)
}

// ToUTF8WithErr converts content to UTF8 encoding
func ToUTF8WithErr(content []byte) (string, error) {
	charsetLabel, err := DetectEncoding(content)
	if err != nil {
		return "", err
	} else if charsetLabel == "UTF-8" {
		return string(RemoveBOMIfPresent(content)), nil
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return string(content), fmt.Errorf("Unknown encoding: %s", charsetLabel)
	}

	// If there is an error, we concatenate the nicely decoded part and the
	// original left over. This way we won't lose much data.
	result, n, err := transform.Bytes(encoding.NewDecoder(), content)
	if err != nil {
		result = append(result, content[n:]...)
	}

	result = RemoveBOMIfPresent(result)

	return string(result), err
}

// ToUTF8WithFallback detects the encoding of content and coverts to UTF-8 if possible
func ToUTF8WithFallback(content []byte) []byte {
	bs, _ := io.ReadAll(ToUTF8WithFallbackReader(bytes.NewReader(content)))
	return bs
}

// ToUTF8 converts content to UTF8 encoding and ignore error
func ToUTF8(content string) string {
	res, _ := ToUTF8WithErr([]byte(content))
	return res
}

// ToUTF8DropErrors makes sure the return string is valid utf-8; attempts conversion if possible
func ToUTF8DropErrors(content []byte) []byte {
	charsetLabel, err := DetectEncoding(content)
	if err != nil || charsetLabel == "UTF-8" {
		return RemoveBOMIfPresent(content)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
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

	return RemoveBOMIfPresent(decoded)
}

// RemoveBOMIfPresent removes a UTF-8 BOM from a []byte
func RemoveBOMIfPresent(content []byte) []byte {
	if len(content) > 2 && bytes.Equal(content[0:3], UTF8BOM) {
		return content[3:]
	}
	return content
}

// DetectEncoding detect the encoding of content
func DetectEncoding(content []byte) (string, error) {
	if utf8.Valid(content) {
		log.Debug("Detected encoding: utf-8 (fast)")
		return "UTF-8", nil
	}

	textDetector := chardet.NewTextDetector()
	var detectContent []byte
	if len(content) < 1024 {
		// Check if original content is valid
		if _, err := textDetector.DetectBest(content); err != nil {
			return "", err
		}
		times := 1024 / len(content)
		detectContent = make([]byte, 0, times*len(content))
		for i := 0; i < times; i++ {
			detectContent = append(detectContent, content...)
		}
	} else {
		detectContent = content
	}

	// Now we can't use DetectBest or just results[0] because the result isn't stable - so we need a tie break
	results, err := textDetector.DetectAll(detectContent)
	if err != nil {
		if err == chardet.NotDetectedError && len(setting.Repository.AnsiCharset) > 0 {
			log.Debug("Using default AnsiCharset: %s", setting.Repository.AnsiCharset)
			return setting.Repository.AnsiCharset, nil
		}
		return "", err
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

		// Otherwise check if this results is earlier in the DetectedCharsetOrder than our current top guesss
		resultPriority, resultHas := setting.Repository.DetectedCharsetScore[strings.ToLower(strings.TrimSpace(result.Charset))]
		if resultHas && (!has || resultPriority < priority) {
			topResult = result
			priority = resultPriority
			has = true
		}
	}

	// FIXME: to properly decouple this function the fallback ANSI charset should be passed as an argument
	if topResult.Charset != "UTF-8" && len(setting.Repository.AnsiCharset) > 0 {
		log.Debug("Using default AnsiCharset: %s", setting.Repository.AnsiCharset)
		return setting.Repository.AnsiCharset, err
	}

	log.Debug("Detected encoding: %s", topResult.Charset)
	return topResult.Charset, err
}
