package chardet

import (
	"bytes"
)

var utf8Bom = []byte{0xEF, 0xBB, 0xBF}

type recognizerUtf8 struct {
}

func newRecognizer_utf8() *recognizerUtf8 {
	return &recognizerUtf8{}
}

func (*recognizerUtf8) Match(input *recognizerInput) (output recognizerOutput) {
	output = recognizerOutput{
		Charset: "UTF-8",
	}
	hasBom := bytes.HasPrefix(input.raw, utf8Bom)
	inputLen := len(input.raw)
	var numValid, numInvalid uint32
	var trailBytes uint8
	for i := 0; i < inputLen; i++ {
		c := input.raw[i]
		if c&0x80 == 0 {
			continue
		}
		if c&0xE0 == 0xC0 {
			trailBytes = 1
		} else if c&0xF0 == 0xE0 {
			trailBytes = 2
		} else if c&0xF8 == 0xF0 {
			trailBytes = 3
		} else {
			numInvalid++
			if numInvalid > 5 {
				break
			}
			trailBytes = 0
		}

		for i++; i < inputLen; i++ {
			c = input.raw[i]
			if c&0xC0 != 0x80 {
				numInvalid++
				break
			}
			if trailBytes--; trailBytes == 0 {
				numValid++
				break
			}
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
	} else if numValid == 0 && numInvalid == 0 {
		// Plain ASCII
		output.Confidence = 10
	} else if numValid > numInvalid*10 {
		output.Confidence = 25
	}
	return
}
