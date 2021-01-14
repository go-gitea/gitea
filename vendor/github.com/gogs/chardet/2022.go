package chardet

import (
	"bytes"
)

type recognizer2022 struct {
	charset string
	escapes [][]byte
}

func (r *recognizer2022) Match(input *recognizerInput) (output recognizerOutput) {
	return recognizerOutput{
		Charset:    r.charset,
		Confidence: r.matchConfidence(input.input),
	}
}

func (r *recognizer2022) matchConfidence(input []byte) int {
	var hits, misses, shifts int
input:
	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == 0x1B {
			for _, esc := range r.escapes {
				if bytes.HasPrefix(input[i+1:], esc) {
					hits++
					i += len(esc)
					continue input
				}
			}
			misses++
		} else if c == 0x0E || c == 0x0F {
			shifts++
		}
	}
	if hits == 0 {
		return 0
	}
	quality := (100*hits - 100*misses) / (hits + misses)
	if hits+shifts < 5 {
		quality -= (5 - (hits + shifts)) * 10
	}
	if quality < 0 {
		quality = 0
	}
	return quality
}

var escapeSequences_2022JP = [][]byte{
	{0x24, 0x28, 0x43}, // KS X 1001:1992
	{0x24, 0x28, 0x44}, // JIS X 212-1990
	{0x24, 0x40},       // JIS C 6226-1978
	{0x24, 0x41},       // GB 2312-80
	{0x24, 0x42},       // JIS X 208-1983
	{0x26, 0x40},       // JIS X 208 1990, 1997
	{0x28, 0x42},       // ASCII
	{0x28, 0x48},       // JIS-Roman
	{0x28, 0x49},       // Half-width katakana
	{0x28, 0x4a},       // JIS-Roman
	{0x2e, 0x41},       // ISO 8859-1
	{0x2e, 0x46},       // ISO 8859-7
}

var escapeSequences_2022KR = [][]byte{
	{0x24, 0x29, 0x43},
}

var escapeSequences_2022CN = [][]byte{
	{0x24, 0x29, 0x41}, // GB 2312-80
	{0x24, 0x29, 0x47}, // CNS 11643-1992 Plane 1
	{0x24, 0x2A, 0x48}, // CNS 11643-1992 Plane 2
	{0x24, 0x29, 0x45}, // ISO-IR-165
	{0x24, 0x2B, 0x49}, // CNS 11643-1992 Plane 3
	{0x24, 0x2B, 0x4A}, // CNS 11643-1992 Plane 4
	{0x24, 0x2B, 0x4B}, // CNS 11643-1992 Plane 5
	{0x24, 0x2B, 0x4C}, // CNS 11643-1992 Plane 6
	{0x24, 0x2B, 0x4D}, // CNS 11643-1992 Plane 7
	{0x4e},             // SS2
	{0x4f},             // SS3
}

func newRecognizer_2022JP() *recognizer2022 {
	return &recognizer2022{
		"ISO-2022-JP",
		escapeSequences_2022JP,
	}
}

func newRecognizer_2022KR() *recognizer2022 {
	return &recognizer2022{
		"ISO-2022-KR",
		escapeSequences_2022KR,
	}
}

func newRecognizer_2022CN() *recognizer2022 {
	return &recognizer2022{
		"ISO-2022-CN",
		escapeSequences_2022CN,
	}
}
