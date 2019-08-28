package minify

import (
	"bytes"
	"encoding/base64"
	"net/url"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/strconv"
)

// Epsilon is the closest number to zero that is not considered to be zero.
var Epsilon = 0.00001

// Mediatype minifies a given mediatype by removing all whitespace.
func Mediatype(b []byte) []byte {
	j := 0
	start := 0
	inString := false
	for i, c := range b {
		if !inString && parse.IsWhitespace(c) {
			if start != 0 {
				j += copy(b[j:], b[start:i])
			} else {
				j += i
			}
			start = i + 1
		} else if c == '"' {
			inString = !inString
		}
	}
	if start != 0 {
		j += copy(b[j:], b[start:])
		return parse.ToLower(b[:j])
	}
	return parse.ToLower(b)
}

// DataURI minifies a data URI and calls a minifier by the specified mediatype. Specifications: https://www.ietf.org/rfc/rfc2397.txt.
func DataURI(m *M, dataURI []byte) []byte {
	if mediatype, data, err := parse.DataURI(dataURI); err == nil {
		dataURI, _ = m.Bytes(string(mediatype), data)
		base64Len := len(";base64") + base64.StdEncoding.EncodedLen(len(dataURI))
		asciiLen := len(dataURI)
		for _, c := range dataURI {
			if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' || c == ' ' {
				asciiLen++
			} else {
				asciiLen += 2
			}
			if asciiLen > base64Len {
				break
			}
		}
		if asciiLen > base64Len {
			encoded := make([]byte, base64Len-len(";base64"))
			base64.StdEncoding.Encode(encoded, dataURI)
			dataURI = encoded
			mediatype = append(mediatype, []byte(";base64")...)
		} else {
			dataURI = []byte(url.QueryEscape(string(dataURI)))
			dataURI = bytes.Replace(dataURI, []byte("\""), []byte("\\\""), -1)
		}
		if len("text/plain") <= len(mediatype) && parse.EqualFold(mediatype[:len("text/plain")], []byte("text/plain")) {
			mediatype = mediatype[len("text/plain"):]
		}
		for i := 0; i+len(";charset=us-ascii") <= len(mediatype); i++ {
			// must start with semicolon and be followed by end of mediatype or semicolon
			if mediatype[i] == ';' && parse.EqualFold(mediatype[i+1:i+len(";charset=us-ascii")], []byte("charset=us-ascii")) && (i+len(";charset=us-ascii") >= len(mediatype) || mediatype[i+len(";charset=us-ascii")] == ';') {
				mediatype = append(mediatype[:i], mediatype[i+len(";charset=us-ascii"):]...)
				break
			}
		}
		dataURI = append(append(append([]byte("data:"), mediatype...), ','), dataURI...)
	}
	return dataURI
}

const MaxInt = int(^uint(0) >> 1)
const MinInt = -MaxInt - 1

// Decimal minifies a given byte slice containing a number (see parse.Number) and removes superfluous characters.
// It does not parse or output exponents.
func Decimal(num []byte, prec int) []byte {
	// omit first + and register mantissa start and end, whether it's negative and the exponent
	neg := false
	start := 0
	dot := -1
	end := len(num)
	if 0 < end && (num[0] == '+' || num[0] == '-') {
		if num[0] == '-' {
			neg = true
		}
		start++
	}
	for i, c := range num[start:] {
		if c == '.' {
			dot = start + i
			break
		}
	}
	if dot == -1 {
		dot = end
	}

	// trim leading zeros but leave at least one digit
	for start < end-1 && num[start] == '0' {
		start++
	}
	// trim trailing zeros
	i := end - 1
	for ; i > dot; i-- {
		if num[i] != '0' {
			end = i + 1
			break
		}
	}
	if i == dot {
		end = dot
		if start == end {
			num[start] = '0'
			return num[start : start+1]
		}
	} else if start == end-1 && num[start] == '0' {
		return num[start:end]
	}

	// apply precision
	if prec > -1 && dot+1+prec < end {
		end = dot + 1 + prec
		inc := num[end] >= '5'
		if inc || num[end-1] == '0' {
			// process either an increase from a lesser significant decimal (>= 5)
			// or remove trailing zeros after the dot, or both
			for i := end - 1; i > start; i-- {
				if i == dot {
					end--
				} else if inc {
					if num[i] == '9' {
						if i > dot {
							end--
						} else {
							num[i] = '0'
							break
						}
					} else {
						num[i]++
						inc = false
						break
					}
				} else if i > dot && num[i] == '0' {
					end--
				} else {
					break
				}
			}
		}
		if dot == start && end == start+1 {
			if inc {
				num[start] = '1'
			} else {
				num[start] = '0'
			}
		} else {
			if dot+1 == end {
				end--
			}
			if inc {
				if num[start] == '9' {
					num[start] = '0'
					copy(num[start+1:], num[start:end])
					end++
					num[start] = '1'
				} else {
					num[start]++
				}
			}
		}
	}

	if neg {
		start--
		num[start] = '-'
	}
	return num[start:end]
}

// Number minifies a given byte slice containing a number (see parse.Number) and removes superfluous characters.
func Number(num []byte, prec int) []byte {
	// omit first + and register mantissa start and end, whether it's negative and the exponent
	neg := false
	start := 0
	dot := -1
	end := len(num)
	origExp := 0
	if 0 < end && (num[0] == '+' || num[0] == '-') {
		if num[0] == '-' {
			neg = true
		}
		start++
	}
	for i, c := range num[start:] {
		if c == '.' {
			dot = start + i
		} else if c == 'e' || c == 'E' {
			end = start + i
			i += start + 1
			if i < len(num) && num[i] == '+' {
				i++
			}
			if tmpOrigExp, n := strconv.ParseInt(num[i:]); n > 0 && tmpOrigExp >= int64(MinInt) && tmpOrigExp <= int64(MaxInt) {
				// range checks for when int is 32 bit
				origExp = int(tmpOrigExp)
			} else {
				return num
			}
			break
		}
	}
	if dot == -1 {
		dot = end
	}

	// trim leading zeros but leave at least one digit
	for start < end-1 && num[start] == '0' {
		start++
	}
	// trim trailing zeros
	i := end - 1
	for ; i > dot; i-- {
		if num[i] != '0' {
			end = i + 1
			break
		}
	}
	if i == dot {
		end = dot
		if start == end {
			num[start] = '0'
			return num[start : start+1]
		}
	} else if start == end-1 && num[start] == '0' {
		return num[start:end]
	}

	// n is the number of significant digits
	// normExp would be the exponent if it were normalised (0.1 <= f < 1)
	n := 0
	normExp := 0
	if dot == start {
		for i = dot + 1; i < end; i++ {
			if num[i] != '0' {
				n = end - i
				normExp = dot - i + 1
				break
			}
		}
	} else if dot == end {
		normExp = end - start
		for i = end - 1; i >= start; i-- {
			if num[i] != '0' {
				n = i + 1 - start
				end = i + 1
				break
			}
		}
	} else {
		n = end - start - 1
		normExp = dot - start
	}

	if origExp < 0 && (normExp < MinInt-origExp || normExp-n < MinInt-origExp) || origExp > 0 && (normExp > MaxInt-origExp || normExp-n > MaxInt-origExp) {
		return num
	}
	normExp += origExp

	// intExp would be the exponent if it were an integer
	intExp := normExp - n
	lenIntExp := 1
	if intExp <= -10 || intExp >= 10 {
		lenIntExp = strconv.LenInt(int64(intExp))
	}

	// there are three cases to consider when printing the number
	// case 1: without decimals and with an exponent (large numbers)
	// case 2: with decimals and without an exponent (around zero)
	// case 3: without decimals and with a negative exponent (small numbers)
	if normExp >= n {
		// case 1
		if dot < end {
			if dot == start {
				start = end - n
			} else {
				// TODO: copy the other part if shorter?
				copy(num[dot:], num[dot+1:end])
				end--
			}
		}
		if normExp >= n+3 {
			num[end] = 'e'
			end++
			for i := end + lenIntExp - 1; i >= end; i-- {
				num[i] = byte(intExp%10) + '0'
				intExp /= 10
			}
			end += lenIntExp
		} else if normExp == n+2 {
			num[end] = '0'
			num[end+1] = '0'
			end += 2
		} else if normExp == n+1 {
			num[end] = '0'
			end++
		}
	} else if normExp >= -lenIntExp-1 {
		// case 2
		zeroes := -normExp
		newDot := 0
		if zeroes > 0 {
			// dot placed at the front and add zeroes
			newDot = end - n - zeroes - 1
			if newDot != dot {
				d := start - newDot
				if d > 0 {
					if dot < end {
						// copy original digits behind the dot backwards
						copy(num[dot+1+d:], num[dot+1:end])
						if dot > start {
							// copy original digits before the dot backwards
							copy(num[start+d+1:], num[start:dot])
						}
					} else if dot > start {
						// copy original digits before the dot backwards
						copy(num[start+d:], num[start:dot])
					}
					newDot = start
					end += d
				} else {
					start += -d
				}
				num[newDot] = '.'
				for i := 0; i < zeroes; i++ {
					num[newDot+1+i] = '0'
				}
			}
		} else {
			// placed in the middle
			if dot == start {
				// TODO: try if placing at the end reduces copying
				// when there are zeroes after the dot
				dot = end - n - 1
				start = dot
			} else if dot >= end {
				// TODO: try if placing at the start reduces copying
				// when input has no dot in it
				dot = end
				end++
			}
			newDot = start + normExp
			if newDot > dot {
				// copy digits forwards
				copy(num[dot:], num[dot+1:newDot+1])
			} else if newDot < dot {
				// copy digits backwards
				copy(num[newDot+1:], num[newDot:dot])
			}
			num[newDot] = '.'
		}

		// apply precision
		dot = newDot
		if prec > -1 && dot+1+prec < end {
			end = dot + 1 + prec
			inc := num[end] >= '5'
			if inc || num[end-1] == '0' {
				for i := end - 1; i > start; i-- {
					if i == dot {
						end--
					} else if inc {
						if num[i] == '9' {
							if i > dot {
								end--
							} else {
								num[i] = '0'
								break
							}
						} else {
							num[i]++
							inc = false
							break
						}
					} else if i > dot && num[i] == '0' {
						end--
					} else {
						break
					}
				}
			}
			if dot == start && end == start+1 {
				if inc {
					num[start] = '1'
				} else {
					num[start] = '0'
				}
			} else {
				if dot+1 == end {
					end--
				}
				if inc {
					if num[start] == '9' {
						num[start] = '0'
						copy(num[start+1:], num[start:end])
						end++
						num[start] = '1'
					} else {
						num[start]++
					}
				}
			}
		}
	} else {
		// case 3

		// find new end, considering moving numbers to the front, removing the dot and increasing the length of the exponent
		newEnd := end
		if dot == start {
			newEnd = start + n
		} else {
			newEnd--
		}
		newEnd += 2 + lenIntExp

		exp := intExp
		lenExp := lenIntExp
		if newEnd < len(num) {
			// it saves space to convert the decimal to an integer and decrease the exponent
			if dot < end {
				if dot == start {
					copy(num[start:], num[end-n:end])
					end = start + n
				} else {
					copy(num[dot:], num[dot+1:end])
					end--
				}
			}
		} else {
			// it does not save space and will panic, so we revert to the original representation
			exp = origExp
			lenExp = 1
			if origExp <= -10 || origExp >= 10 {
				lenExp = strconv.LenInt(int64(origExp))
			}
		}
		num[end] = 'e'
		num[end+1] = '-'
		end += 2
		exp = -exp
		for i := end + lenExp - 1; i >= end; i-- {
			num[i] = byte(exp%10) + '0'
			exp /= 10
		}
		end += lenExp
	}

	if neg {
		start--
		num[start] = '-'
	}
	return num[start:end]
}
