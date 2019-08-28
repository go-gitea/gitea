// Package parse contains a collection of parsers for various formats in its subpackages.
package parse

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/url"
)

// ErrBadDataURI is returned by DataURI when the byte slice does not start with 'data:' or is too short.
var ErrBadDataURI = errors.New("not a data URI")

// Number returns the number of bytes that parse as a number of the regex format (+|-)?([0-9]+(\.[0-9]+)?|\.[0-9]+)((e|E)(+|-)?[0-9]+)?.
func Number(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	i := 0
	if b[i] == '+' || b[i] == '-' {
		i++
		if i >= len(b) {
			return 0
		}
	}
	firstDigit := (b[i] >= '0' && b[i] <= '9')
	if firstDigit {
		i++
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
		}
	}
	if i < len(b) && b[i] == '.' {
		i++
		if i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
			for i < len(b) && b[i] >= '0' && b[i] <= '9' {
				i++
			}
		} else if firstDigit {
			// . could belong to the next token
			i--
			return i
		} else {
			return 0
		}
	} else if !firstDigit {
		return 0
	}
	iOld := i
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || b[i] < '0' || b[i] > '9' {
			// e could belong to next token
			return iOld
		}
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			i++
		}
	}
	return i
}

// Dimension parses a byte-slice and returns the length of the number and its unit.
func Dimension(b []byte) (int, int) {
	num := Number(b)
	if num == 0 || num == len(b) {
		return num, 0
	} else if b[num] == '%' {
		return num, 1
	} else if b[num] >= 'a' && b[num] <= 'z' || b[num] >= 'A' && b[num] <= 'Z' {
		i := num + 1
		for i < len(b) && (b[i] >= 'a' && b[i] <= 'z' || b[i] >= 'A' && b[i] <= 'Z') {
			i++
		}
		return num, i - num
	}
	return num, 0
}

// Mediatype parses a given mediatype and splits the mimetype from the parameters.
// It works similar to mime.ParseMediaType but is faster.
func Mediatype(b []byte) ([]byte, map[string]string) {
	i := 0
	for i < len(b) && b[i] == ' ' {
		i++
	}
	b = b[i:]
	n := len(b)
	mimetype := b
	var params map[string]string
	for i := 3; i < n; i++ { // mimetype is at least three characters long
		if b[i] == ';' || b[i] == ' ' {
			mimetype = b[:i]
			if b[i] == ' ' {
				i++
				for i < n && b[i] == ' ' {
					i++
				}
				if i < n && b[i] != ';' {
					break
				}
			}
			params = map[string]string{}
			s := string(b)
		PARAM:
			i++
			for i < n && s[i] == ' ' {
				i++
			}
			start := i
			for i < n && s[i] != '=' && s[i] != ';' && s[i] != ' ' {
				i++
			}
			key := s[start:i]
			for i < n && s[i] == ' ' {
				i++
			}
			if i < n && s[i] == '=' {
				i++
				for i < n && s[i] == ' ' {
					i++
				}
				start = i
				for i < n && s[i] != ';' && s[i] != ' ' {
					i++
				}
			} else {
				start = i
			}
			params[key] = s[start:i]
			for i < n && s[i] == ' ' {
				i++
			}
			if i < n && s[i] == ';' {
				goto PARAM
			}
			break
		}
	}
	return mimetype, params
}

// DataURI parses the given data URI and returns the mediatype, data and ok.
func DataURI(dataURI []byte) ([]byte, []byte, error) {
	if len(dataURI) > 5 && bytes.Equal(dataURI[:5], []byte("data:")) {
		dataURI = dataURI[5:]
		inBase64 := false
		var mediatype []byte
		i := 0
		for j := 0; j < len(dataURI); j++ {
			c := dataURI[j]
			if c == '=' || c == ';' || c == ',' {
				if c != '=' && bytes.Equal(TrimWhitespace(dataURI[i:j]), []byte("base64")) {
					if len(mediatype) > 0 {
						mediatype = mediatype[:len(mediatype)-1]
					}
					inBase64 = true
					i = j
				} else if c != ',' {
					mediatype = append(append(mediatype, TrimWhitespace(dataURI[i:j])...), c)
					i = j + 1
				} else {
					mediatype = append(mediatype, TrimWhitespace(dataURI[i:j])...)
				}
				if c == ',' {
					if len(mediatype) == 0 || mediatype[0] == ';' {
						mediatype = []byte("text/plain")
					}
					data := dataURI[j+1:]
					if inBase64 {
						decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
						n, err := base64.StdEncoding.Decode(decoded, data)
						if err != nil {
							return nil, nil, err
						}
						data = decoded[:n]
					} else if unescaped, err := url.QueryUnescape(string(data)); err == nil {
						data = []byte(unescaped)
					}
					return mediatype, data, nil
				}
			}
		}
	}
	return nil, nil, ErrBadDataURI
}

// QuoteEntity parses the given byte slice and returns the quote that got matched (' or ") and its entity length.
func QuoteEntity(b []byte) (quote byte, n int) {
	if len(b) < 5 || b[0] != '&' {
		return 0, 0
	}
	if b[1] == '#' {
		if b[2] == 'x' {
			i := 3
			for i < len(b) && b[i] == '0' {
				i++
			}
			if i+2 < len(b) && b[i] == '2' && b[i+2] == ';' {
				if b[i+1] == '2' {
					return '"', i + 3 // &#x22;
				} else if b[i+1] == '7' {
					return '\'', i + 3 // &#x27;
				}
			}
		} else {
			i := 2
			for i < len(b) && b[i] == '0' {
				i++
			}
			if i+2 < len(b) && b[i] == '3' && b[i+2] == ';' {
				if b[i+1] == '4' {
					return '"', i + 3 // &#34;
				} else if b[i+1] == '9' {
					return '\'', i + 3 // &#39;
				}
			}
		}
	} else if len(b) >= 6 && b[5] == ';' {
		if EqualFold(b[1:5], []byte{'q', 'u', 'o', 't'}) {
			return '"', 6 // &quot;
		} else if EqualFold(b[1:5], []byte{'a', 'p', 'o', 's'}) {
			return '\'', 6 // &apos;
		}
	}
	return 0, 0
}
