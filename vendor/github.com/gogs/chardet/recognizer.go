package chardet

type recognizer interface {
	Match(*recognizerInput) recognizerOutput
}

type recognizerOutput Result

type recognizerInput struct {
	raw         []byte
	input       []byte
	tagStripped bool
	byteStats   []int
	hasC1Bytes  bool
}

func newRecognizerInput(raw []byte, stripTag bool) *recognizerInput {
	input, stripped := mayStripInput(raw, stripTag)
	byteStats := computeByteStats(input)
	return &recognizerInput{
		raw:         raw,
		input:       input,
		tagStripped: stripped,
		byteStats:   byteStats,
		hasC1Bytes:  computeHasC1Bytes(byteStats),
	}
}

func mayStripInput(raw []byte, stripTag bool) (out []byte, stripped bool) {
	const inputBufferSize = 8192
	out = make([]byte, 0, inputBufferSize)
	var badTags, openTags int32
	var inMarkup bool = false
	stripped = false
	if stripTag {
		stripped = true
		for _, c := range raw {
			if c == '<' {
				if inMarkup {
					badTags += 1
				}
				inMarkup = true
				openTags += 1
			}
			if !inMarkup {
				out = append(out, c)
				if len(out) >= inputBufferSize {
					break
				}
			}
			if c == '>' {
				inMarkup = false
			}
		}
	}
	if openTags < 5 || openTags/5 < badTags || (len(out) < 100 && len(raw) > 600) {
		limit := len(raw)
		if limit > inputBufferSize {
			limit = inputBufferSize
		}
		out = make([]byte, limit)
		copy(out, raw[:limit])
		stripped = false
	}
	return
}

func computeByteStats(input []byte) []int {
	r := make([]int, 256)
	for _, c := range input {
		r[c] += 1
	}
	return r
}

func computeHasC1Bytes(byteStats []int) bool {
	for _, count := range byteStats[0x80 : 0x9F+1] {
		if count > 0 {
			return true
		}
	}
	return false
}
