package css

import (
	"encoding/hex"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

func removeMarkupNewlines(data []byte) []byte {
	// remove any \\\r\n \\\r \\\n
	for i := 1; i < len(data)-2; i++ {
		if data[i] == '\\' && (data[i+1] == '\n' || data[i+1] == '\r') {
			// encountered first replacee, now start to move bytes to the front
			j := i + 2
			if data[i+1] == '\r' && len(data) > i+2 && data[i+2] == '\n' {
				j++
			}
			for ; j < len(data); j++ {
				if data[j] == '\\' && len(data) > j+1 && (data[j+1] == '\n' || data[j+1] == '\r') {
					if data[j+1] == '\r' && len(data) > j+2 && data[j+2] == '\n' {
						j++
					}
					j++
				} else {
					data[i] = data[j]
					i++
				}
			}
			data = data[:i]
			break
		}
	}
	return data
}

func rgbToToken(r, g, b float64) Token {
	// r, g, b are in interval [0.0, 1.0]
	rgb := []byte{byte((r * 255.0) + 0.5), byte((g * 255.0) + 0.5), byte((b * 255.0) + 0.5)}

	val := make([]byte, 7)
	val[0] = '#'
	hex.Encode(val[1:], rgb)
	parse.ToLower(val)
	if s, ok := ShortenColorHex[string(val[:7])]; ok {
		return Token{css.IdentToken, s, nil, 0, 0}
	} else if val[1] == val[2] && val[3] == val[4] && val[5] == val[6] {
		val[2] = val[3]
		val[3] = val[5]
		val = val[:4]
	} else {
		val = val[:7]
	}
	return Token{css.HashToken, val, nil, 0, 0}
}
