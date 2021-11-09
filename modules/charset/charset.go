// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package charset

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/gogs/chardet"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/bidi"
)

// UTF8BOM is the utf-8 byte-order marker
var UTF8BOM = []byte{'\xef', '\xbb', '\xbf'}

// EscapeStatus represents the findings of the unicode escaper
type EscapeStatus struct {
	Escaped      bool
	HasError     bool
	HasBadRunes  bool
	HasControls  bool
	HasSpaces    bool
	HasMarks     bool
	HasBIDI      bool
	BadBIDI      bool
	HasRTLScript bool
	HasLTRScript bool
}

// Or combines two EscapeStatus structs into one representing the conjunction of the two
func (status EscapeStatus) Or(other EscapeStatus) EscapeStatus {
	status.Escaped = status.Escaped || other.Escaped
	status.HasError = status.HasError || other.HasError
	status.HasBadRunes = status.HasBadRunes || other.HasBadRunes
	status.HasControls = status.HasControls || other.HasControls
	status.HasSpaces = status.HasSpaces || other.HasSpaces
	status.HasMarks = status.HasMarks || other.HasMarks
	status.HasBIDI = status.HasBIDI || other.HasBIDI
	status.BadBIDI = status.BadBIDI || other.BadBIDI
	status.HasRTLScript = status.HasRTLScript || other.HasRTLScript
	status.HasLTRScript = status.HasLTRScript || other.HasLTRScript
	return status
}

// EscapeControlString escapes the unicode control sequences in a provided string and returns the findings as an EscapeStatus and the escaped string
func EscapeControlString(text string) (EscapeStatus, string) {
	sb := &strings.Builder{}
	escaped, _ := EscapeControlReader(strings.NewReader(text), sb)
	return escaped, sb.String()
}

// EscapeControlBytes escapes the unicode control sequences  a provided []byte and returns the findings as an EscapeStatus and the escaped []byte
func EscapeControlBytes(text []byte) (EscapeStatus, []byte) {
	buf := &bytes.Buffer{}
	escaped, _ := EscapeControlReader(bytes.NewReader(text), buf)
	return escaped, buf.Bytes()
}

// EscapeControlReader escapes the unicode control sequences  a provided Reader writing the escaped output to the output and returns the findings as an EscapeStatus and an error
func EscapeControlReader(text io.Reader, output io.Writer) (escaped EscapeStatus, err error) {
	buf := make([]byte, 4096)
	readStart := 0
	var n int
	var writePos int

	lineHasBIDI := false
	lineHasRTLScript := false
	lineHasLTRScript := false

readingloop:
	for err == nil {
		n, err = text.Read(buf[readStart:])
		bs := buf[:n+readStart]
		i := 0

		for i < len(bs) {
			r, size := utf8.DecodeRune(bs[i:])
			// Now handle the codepoints
			switch {
			case r == utf8.RuneError:
				if writePos < i {
					if _, err = output.Write(bs[writePos:i]); err != nil {
						escaped.HasError = true
						return
					}
					writePos = i
				}
				// runes can be at most 4 bytes - so...
				if len(bs)-i <= 3 {
					// if not request more data
					copy(buf, bs[i:])
					readStart = n - i
					writePos = 0
					continue readingloop
				}
				// this is a real broken rune
				escaped.HasBadRunes = true
				escaped.Escaped = true
				if _, err = fmt.Fprintf(output, `<span class="broken-code-point">&lt;%X&gt;</span>`, bs[i:i+size]); err != nil {
					escaped.HasError = true
					return
				}
				writePos += size
			case r == '\n':
				if lineHasBIDI && !lineHasRTLScript && lineHasLTRScript {
					escaped.BadBIDI = true
				}
				lineHasBIDI = false
				lineHasRTLScript = false
				lineHasLTRScript = false

			case r == '\r' || r == '\t' || r == ' ':
				// These are acceptable control characters and space characters
			case unicode.IsSpace(r):
				escaped.HasSpaces = true
				escaped.Escaped = true
				if writePos < i {
					if _, err = output.Write(bs[writePos:i]); err != nil {
						escaped.HasError = true
						return
					}
				}
				if _, err = fmt.Fprintf(output, `<span class="escaped-code-point" escaped="[U+%04X]"><span class="char">%c</span></span>`, r, r); err != nil {
					escaped.HasError = true
					return
				}
				writePos = i + size
			case unicode.Is(unicode.Bidi_Control, r):
				escaped.Escaped = true
				escaped.HasBIDI = true
				if writePos < i {
					if _, err = output.Write(bs[writePos:i]); err != nil {
						escaped.HasError = true
						return
					}
				}
				lineHasBIDI = true
				if _, err = fmt.Fprintf(output, `<span class="escaped-code-point" escaped="[U+%04X]"><span class="char">%c</span></span>`, r, r); err != nil {
					escaped.HasError = true
					return
				}
				writePos = i + size
			case unicode.Is(unicode.C, r):
				escaped.Escaped = true
				escaped.HasControls = true
				if writePos < i {
					if _, err = output.Write(bs[writePos:i]); err != nil {
						escaped.HasError = true
						return
					}
				}
				if _, err = fmt.Fprintf(output, `<span class="escaped-code-point" escaped="[U+%04X]"><span class="char">%c</span></span>`, r, r); err != nil {
					escaped.HasError = true
					return
				}
				writePos = i + size
			case unicode.Is(unicode.M, r):
				escaped.Escaped = true
				escaped.HasMarks = true
				if writePos < i {
					if _, err = output.Write(bs[writePos:i]); err != nil {
						escaped.HasError = true
						return
					}
				}
				if _, err = fmt.Fprintf(output, `<span class="escaped-code-point" escaped="[U+%04X]"><span class="char">%c</span></span>`, r, r); err != nil {
					escaped.HasError = true
					return
				}
				writePos = i + size
			default:
				p, _ := bidi.Lookup(bs[i : i+size])
				c := p.Class()
				if c == bidi.R || c == bidi.AL {
					lineHasRTLScript = true
					escaped.HasRTLScript = true
				} else if c == bidi.L {
					lineHasLTRScript = true
					escaped.HasLTRScript = true
				}
			}
			i += size
		}
		if n > 0 {
			// we read something...
			// write everything unwritten
			if writePos < i {
				if _, err = output.Write(bs[writePos:i]); err != nil {
					escaped.HasError = true
					return
				}
			}

			// reset the starting positions for the next read
			readStart = 0
			writePos = 0
		}
	}
	if readStart > 0 {
		// this means that there is an incomplete or broken rune at 0-readStart and we read nothing on the last go round
		escaped.Escaped = true
		escaped.HasBadRunes = true
		if _, err = fmt.Fprintf(output, `<span class="broken-code-point">&lt;%X&gt;</span>`, buf[:readStart]); err != nil {
			escaped.HasError = true
			return
		}
	}
	if err == io.EOF {
		if lineHasBIDI && !lineHasRTLScript && lineHasLTRScript {
			escaped.BadBIDI = true
		}
		err = nil
		return
	}
	escaped.HasError = true
	return
}

// ToUTF8WithFallbackReader detects the encoding of content and coverts to UTF-8 reader if possible
func ToUTF8WithFallbackReader(rd io.Reader) io.Reader {
	var buf = make([]byte, 2048)
	n, err := util.ReadAtMost(rd, buf)
	if err != nil {
		return io.MultiReader(bytes.NewReader(RemoveBOMIfPresent(buf[:n])), rd)
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
