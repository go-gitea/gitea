// Copyright 2021 The Gitea Authors. All rights reserved.
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

	"golang.org/x/text/unicode/bidi"
)

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
	st := status
	st.Escaped = st.Escaped || other.Escaped
	st.HasError = st.HasError || other.HasError
	st.HasBadRunes = st.HasBadRunes || other.HasBadRunes
	st.HasControls = st.HasControls || other.HasControls
	st.HasSpaces = st.HasSpaces || other.HasSpaces
	st.HasMarks = st.HasMarks || other.HasMarks
	st.HasBIDI = st.HasBIDI || other.HasBIDI
	st.BadBIDI = st.BadBIDI || other.BadBIDI
	st.HasRTLScript = st.HasRTLScript || other.HasRTLScript
	st.HasLTRScript = st.HasLTRScript || other.HasLTRScript
	return st
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
				if err = writeBroken(output, bs[i:i+size]); err != nil {
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
				if err = writeEscaped(output, r); err != nil {
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
				if err = writeEscaped(output, r); err != nil {
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
				if err = writeEscaped(output, r); err != nil {
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
				if err = writeEscaped(output, r); err != nil {
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
		if err = writeBroken(output, buf[:readStart]); err != nil {
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

func writeBroken(output io.Writer, bs []byte) (err error) {
	_, err = fmt.Fprintf(output, `<span class="broken-code-point">&lt;%X&gt;</span>`, bs)
	return
}

func writeEscaped(output io.Writer, r rune) (err error) {
	_, err = fmt.Fprintf(output, `<span class="escaped-code-point" data-escaped="[U+%04X]"><span class="char">%c</span></span>`, r, r)
	return
}
