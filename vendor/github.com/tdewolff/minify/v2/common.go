package minify

import (
	"encoding/base64"

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
	origData := parse.Copy(dataURI)
	mediatype, data, err := parse.DataURI(dataURI)
	if err != nil {
		return dataURI
	}

	data, _ = m.Bytes(string(mediatype), data)
	base64Len := len(";base64") + base64.StdEncoding.EncodedLen(len(data))
	asciiLen := len(data)
	for _, c := range data {
		if parse.URLEncodingTable[c] {
			asciiLen += 2
		}
		if asciiLen > base64Len {
			break
		}
	}
	if len(origData) < base64Len && len(origData) < asciiLen {
		return origData
	}
	if base64Len < asciiLen {
		encoded := make([]byte, base64Len-len(";base64"))
		base64.StdEncoding.Encode(encoded, data)
		data = encoded
		mediatype = append(mediatype, []byte(";base64")...)
	} else {
		data = parse.EncodeURL(data, parse.URLEncodingTable)
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
	return append(append(append([]byte("data:"), mediatype...), ','), data...)
}

const MaxInt = int(^uint(0) >> 1)
const MinInt = -MaxInt - 1

// Decimal minifies a given byte slice containing a number (see parse.Number) and removes superfluous characters.
// It does not parse or output exponents. prec is the number of significant digits. When prec is zero it will keep all digits. Only digits after the dot can be removed to reach the number of significant digits. Very large number may thus have more significant digits.
func Decimal(num []byte, prec int) []byte {
	if len(num) <= 1 {
		return num
	}

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
	for ; dot < i; i-- {
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
	if 0 < prec && dot <= start+prec {
		precEnd := start + prec + 1 // include dot
		if dot == start {           // for numbers like .012
			digit := start + 1
			for digit < end && num[digit] == '0' {
				digit++
			}
			precEnd = digit + prec
		}
		if precEnd < end {
			end = precEnd

			// process either an increase from a lesser significant decimal (>= 5)
			// or remove trailing zeros after the dot, or both
			i := end - 1
			inc := '5' <= num[end]
			for ; start < i; i-- {
				if i == dot {
					// no-op
				} else if inc && num[i] != '9' {
					num[i]++
					inc = false
					break
				} else if inc && i < dot { // end inc for integer
					num[i] = '0'
				} else if !inc && (i < dot || num[i] != '0') {
					break
				}
			}
			if i < dot {
				end = dot
			} else {
				end = i + 1
			}

			if inc {
				if dot == start && end == start+1 {
					num[start] = '1'
				} else if num[start] == '9' {
					num[start] = '1'
					num[start+1] = '0'
					end++
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
	if len(num) <= 1 {
		return num
	}

	// omit first + and register mantissa start and end, whether it's negative and the exponent
	neg := false
	start := 0
	dot := -1
	end := len(num)
	origExp := 0
	if num[0] == '+' || num[0] == '-' {
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
			if tmpOrigExp, n := strconv.ParseInt(num[i:]); 0 < n && int64(MinInt) <= tmpOrigExp && tmpOrigExp <= int64(MaxInt) {
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
	for ; dot < i; i-- {
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
	if 0 < prec { //&& (dot <= start+prec || start+prec+1 < dot || 0 < origExp) { // don't minify 9 to 10, but do 999 to 1e3 and 99e1 to 1e3
		precEnd := start + prec
		if dot == start { // for numbers like .012
			digit := start + 1
			for digit < end && num[digit] == '0' {
				digit++
			}
			precEnd = digit + prec
		} else if dot < precEnd { // for numbers where precision will include the dot
			precEnd++
		}
		if precEnd < end && (dot < end || 1 < dot-precEnd+origExp) { // do not minify 9=>10 or 99=>100 or 9e1=>1e2 (but 90), but 999=>1e3 and 99e1=>1e3
			end = precEnd
			inc := '5' <= num[end]
			if dot == end {
				inc = end+1 < len(num) && '5' <= num[end+1]
			}
			if precEnd < dot {
				origExp += dot - precEnd
				dot = precEnd
			}
			// process either an increase from a lesser significant decimal (>= 5)
			// and remove trailing zeros
			i := end - 1
			for ; start < i; i-- {
				if i == dot {
					// no-op
				} else if inc && num[i] != '9' {
					num[i]++
					inc = false
					break
				} else if !inc && num[i] != '0' {
					break
				}
			}
			end = i + 1
			if end < dot {
				origExp += dot - end
				dot = end
			}
			if inc { // single digit left
				if dot == start {
					num[start] = '1'
					dot = start + 1
				} else if num[start] == '9' {
					num[start] = '1'
					origExp++
				} else {
					num[start]++
				}
			}
		}
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
		for i = end - 1; start <= i; i-- {
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

	if origExp < 0 && (normExp < MinInt-origExp || normExp-n < MinInt-origExp) || 0 < origExp && (MaxInt-origExp < normExp || MaxInt-origExp < normExp-n) {
		return num // exponent overflow
	}
	normExp += origExp

	// intExp would be the exponent if it were an integer
	intExp := normExp - n
	lenIntExp := strconv.LenInt(int64(intExp))
	lenNormExp := strconv.LenInt(int64(normExp))

	// there are three cases to consider when printing the number
	// case 1: without decimals and with a positive exponent (large numbers: 5e4)
	// case 2: with decimals and with a negative exponent (small numbers with many digits: .123456e-4)
	// case 3: with decimals and without an exponent (around zero: 5.6)
	// case 4: without decimals and with a negative exponent (small numbers: 123456e-9)
	if n <= normExp {
		// case 1: print number with positive exponent
		if dot < end {
			// remove dot, either from the front or copy the smallest part
			if dot == start {
				start = end - n
			} else if dot-start < end-dot-1 {
				copy(num[start+1:], num[start:dot])
				start++
			} else {
				copy(num[dot:], num[dot+1:end])
				end--
			}
		}
		if n+3 <= normExp {
			num[end] = 'e'
			end++
			for i := end + lenIntExp - 1; end <= i; i-- {
				num[i] = byte(intExp%10) + '0'
				intExp /= 10
			}
			end += lenIntExp
		} else if n+2 == normExp {
			num[end] = '0'
			num[end+1] = '0'
			end += 2
		} else if n+1 == normExp {
			num[end] = '0'
			end++
		}
	} else if normExp < -3 && lenNormExp < lenIntExp {
		// case 2: print normalized number (0.1 <= f < 1)
		zeroes := -normExp + origExp
		if 0 < zeroes {
			copy(num[start+1:], num[start+1+zeroes:end])
			end -= zeroes
		} else if zeroes < 0 {
			copy(num[start+1:], num[start:dot])
			num[start] = '.'
		}
		num[end] = 'e'
		num[end+1] = '-'
		end += 2
		for i := end + lenNormExp - 1; end <= i; i-- {
			num[i] = -byte(normExp%10) + '0'
			normExp /= 10
		}
		end += lenNormExp
	} else if -lenIntExp-1 <= normExp {
		// case 3: print number without exponent
		zeroes := -normExp
		if 0 < zeroes {
			// dot placed at the front and negative exponent, adding zeroes
			newDot := end - n - zeroes - 1
			if newDot != dot {
				d := start - newDot
				if 0 < d {
					if dot < end {
						// copy original digits after the dot towards the end
						copy(num[dot+1+d:], num[dot+1:end])
						if start < dot {
							// copy original digits before the dot towards the end
							copy(num[start+d+1:], num[start:dot])
						}
					} else if start < dot {
						// copy original digits before the dot towards the end
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
			// dot placed in the middle of the number
			if dot == start {
				// when there are zeroes after the dot
				dot = end - n - 1
				start = dot
			} else if end <= dot {
				// when input has no dot in it
				dot = end
				end++
			}
			newDot := start + normExp
			// move digits between dot and newDot towards the end
			if dot < newDot {
				copy(num[dot:], num[dot+1:newDot+1])
			} else if newDot < dot {
				copy(num[newDot+1:], num[newDot:dot])
			}
			num[newDot] = '.'
		}
	} else {
		// case 4: print number with negative exponent
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
			if origExp <= -10 || 10 <= origExp {
				lenExp = strconv.LenInt(int64(origExp))
			}
		}
		num[end] = 'e'
		num[end+1] = '-'
		end += 2
		for i := end + lenExp - 1; end <= i; i-- {
			num[i] = -byte(exp%10) + '0'
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
