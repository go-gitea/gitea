package chardet

import (
	"bytes"
)

var (
	utf16beBom = []byte{0xFE, 0xFF}
	utf16leBom = []byte{0xFF, 0xFE}
	utf32beBom = []byte{0x00, 0x00, 0xFE, 0xFF}
	utf32leBom = []byte{0xFF, 0xFE, 0x00, 0x00}
)

type recognizerUtf16be struct {
}

func newRecognizer_utf16be() *recognizerUtf16be {
	return &recognizerUtf16be{}
}

func (*recognizerUtf16be) Match(input *recognizerInput) (output recognizerOutput) {
	output = recognizerOutput{
		Charset: "UTF-16BE",
	}
	if bytes.HasPrefix(input.raw, utf16beBom) {
		output.Confidence = 100
	}
	return
}

type recognizerUtf16le struct {
}

func newRecognizer_utf16le() *recognizerUtf16le {
	return &recognizerUtf16le{}
}

func (*recognizerUtf16le) Match(input *recognizerInput) (output recognizerOutput) {
	output = recognizerOutput{
		Charset: "UTF-16LE",
	}
	if bytes.HasPrefix(input.raw, utf16leBom) && !bytes.HasPrefix(input.raw, utf32leBom) {
		output.Confidence = 100
	}
	return
}

type recognizerUtf32 struct {
	name       string
	bom        []byte
	decodeChar func(input []byte) uint32
}

func decodeUtf32be(input []byte) uint32 {
	return uint32(input[0])<<24 | uint32(input[1])<<16 | uint32(input[2])<<8 | uint32(input[3])
}

func decodeUtf32le(input []byte) uint32 {
	return uint32(input[3])<<24 | uint32(input[2])<<16 | uint32(input[1])<<8 | uint32(input[0])
}

func newRecognizer_utf32be() *recognizerUtf32 {
	return &recognizerUtf32{
		"UTF-32BE",
		utf32beBom,
		decodeUtf32be,
	}
}

func newRecognizer_utf32le() *recognizerUtf32 {
	return &recognizerUtf32{
		"UTF-32LE",
		utf32leBom,
		decodeUtf32le,
	}
}

func (r *recognizerUtf32) Match(input *recognizerInput) (output recognizerOutput) {
	output = recognizerOutput{
		Charset: r.name,
	}
	hasBom := bytes.HasPrefix(input.raw, r.bom)
	var numValid, numInvalid uint32
	for b := input.raw; len(b) >= 4; b = b[4:] {
		if c := r.decodeChar(b); c >= 0x10FFFF || (c >= 0xD800 && c <= 0xDFFF) {
			numInvalid++
		} else {
			numValid++
		}
	}
	if hasBom && numInvalid == 0 {
		output.Confidence = 100
	} else if hasBom && numValid > numInvalid*10 {
		output.Confidence = 80
	} else if numValid > 3 && numInvalid == 0 {
		output.Confidence = 100
	} else if numValid > 0 && numInvalid == 0 {
		output.Confidence = 80
	} else if numValid > numInvalid*10 {
		output.Confidence = 25
	}
	return
}
