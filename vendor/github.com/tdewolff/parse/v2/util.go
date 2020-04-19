package parse

import (
	"bytes"
	"strconv"
)

// Copy returns a copy of the given byte slice.
func Copy(src []byte) (dst []byte) {
	dst = make([]byte, len(src))
	copy(dst, src)
	return
}

// ToLower converts all characters in the byte slice from A-Z to a-z.
func ToLower(src []byte) []byte {
	for i, c := range src {
		if c >= 'A' && c <= 'Z' {
			src[i] = c + ('a' - 'A')
		}
	}
	return src
}

// EqualFold returns true when s matches case-insensitively the targetLower (which must be lowercase).
func EqualFold(s, targetLower []byte) bool {
	if len(s) != len(targetLower) {
		return false
	}
	for i, c := range targetLower {
		d := s[i]
		if d != c && (d < 'A' || d > 'Z' || d+('a'-'A') != c) {
			return false
		}
	}
	return true
}

var whitespaceTable = [256]bool{
	// ASCII
	false, false, false, false, false, false, false, false,
	false, true, true, false, true, true, false, false, // tab, new line, form feed, carriage return
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	true, false, false, false, false, false, false, false, // space
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	// non-ASCII
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
}

// IsWhitespace returns true for space, \n, \r, \t, \f.
func IsWhitespace(c byte) bool {
	return whitespaceTable[c]
}

var newlineTable = [256]bool{
	// ASCII
	false, false, false, false, false, false, false, false,
	false, false, true, false, false, true, false, false, // new line, carriage return
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	// non-ASCII
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,

	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
	false, false, false, false, false, false, false, false,
}

// IsNewline returns true for \n, \r.
func IsNewline(c byte) bool {
	return newlineTable[c]
}

// IsAllWhitespace returns true when the entire byte slice consists of space, \n, \r, \t, \f.
func IsAllWhitespace(b []byte) bool {
	for _, c := range b {
		if !IsWhitespace(c) {
			return false
		}
	}
	return true
}

// TrimWhitespace removes any leading and trailing whitespace characters.
func TrimWhitespace(b []byte) []byte {
	n := len(b)
	start := n
	for i := 0; i < n; i++ {
		if !IsWhitespace(b[i]) {
			start = i
			break
		}
	}
	end := n
	for i := n - 1; i >= start; i-- {
		if !IsWhitespace(b[i]) {
			end = i + 1
			break
		}
	}
	return b[start:end]
}

// ReplaceMultipleWhitespace replaces character series of space, \n, \t, \f, \r into a single space or newline (when the serie contained a \n or \r).
func ReplaceMultipleWhitespace(b []byte) []byte {
	j, k := 0, 0 // j is write position, k is start of next text section
	for i := 0; i < len(b); i++ {
		if IsWhitespace(b[i]) {
			start := i
			newline := IsNewline(b[i])
			i++
			for ; i < len(b) && IsWhitespace(b[i]); i++ {
				if IsNewline(b[i]) {
					newline = true
				}
			}
			if newline {
				b[start] = '\n'
			} else {
				b[start] = ' '
			}
			if 1 < i-start { // more than one whitespace
				if j == 0 {
					j = start + 1
				} else {
					j += copy(b[j:], b[k:start+1])
				}
				k = i
			}
		}
	}
	if j == 0 {
		return b
	} else if j == 1 { // only if starts with whitespace
		b[k-1] = b[0]
		return b[k-1:]
	} else if k < len(b) {
		j += copy(b[j:], b[k:])
	}
	return b[:j]
}

// replaceEntities will replace in b at index i, assuming that b[i] == '&' and that i+3<len(b). The returned int will be the last character of the entity, so that the next iteration can safely do i++ to continue and not miss any entitites.
func replaceEntities(b []byte, i int, entitiesMap map[string][]byte, revEntitiesMap map[byte][]byte) ([]byte, int) {
	const MaxEntityLength = 31 // longest HTML entity: CounterClockwiseContourIntegral
	var r []byte
	j := i + 1
	if b[j] == '#' {
		j++
		if b[j] == 'x' {
			j++
			c := 0
			for ; j < len(b) && (b[j] >= '0' && b[j] <= '9' || b[j] >= 'a' && b[j] <= 'f' || b[j] >= 'A' && b[j] <= 'F'); j++ {
				if b[j] <= '9' {
					c = c<<4 + int(b[j]-'0')
				} else if b[j] <= 'F' {
					c = c<<4 + int(b[j]-'A') + 10
				} else if b[j] <= 'f' {
					c = c<<4 + int(b[j]-'a') + 10
				}
			}
			if j <= i+3 || 10000 <= c {
				return b, j - 1
			}
			if c < 128 {
				r = []byte{byte(c)}
			} else {
				r = append(r, '&', '#')
				r = strconv.AppendInt(r, int64(c), 10)
				r = append(r, ';')
			}
		} else {
			c := 0
			for ; j < len(b) && c < 128 && b[j] >= '0' && b[j] <= '9'; j++ {
				c = c*10 + int(b[j]-'0')
			}
			if j <= i+2 || 128 <= c {
				return b, j - 1
			}
			r = []byte{byte(c)}
		}
	} else {
		for ; j < len(b) && j-i-1 <= MaxEntityLength && b[j] != ';'; j++ {
		}
		if j <= i+1 || len(b) <= j {
			return b, j - 1
		}

		var ok bool
		r, ok = entitiesMap[string(b[i+1:j])]
		if !ok {
			return b, j
		}
	}

	// j is at semicolon
	n := j + 1 - i
	if j < len(b) && b[j] == ';' && 2 < n {
		if len(r) == 1 {
			if q, ok := revEntitiesMap[r[0]]; ok {
				if len(q) == len(b[i:j+1]) && bytes.Equal(q, b[i:j+1]) {
					return b, j
				}
				r = q
			} else if r[0] == '&' {
				// check if for example &amp; is followed by something that could potentially be an entity
				k := j + 1
				if k < len(b) && b[k] == '#' {
					k++
				}
				for ; k < len(b) && k-j <= MaxEntityLength && (b[k] >= '0' && b[k] <= '9' || b[k] >= 'a' && b[k] <= 'z' || b[k] >= 'A' && b[k] <= 'Z'); k++ {
				}
				if k < len(b) && b[k] == ';' {
					return b, k
				}
			}
		}

		copy(b[i:], r)
		copy(b[i+len(r):], b[j+1:])
		b = b[:len(b)-n+len(r)]
		return b, i + len(r) - 1
	}
	return b, i
}

// ReplaceEntities replaces all occurrences of entites (such as &quot;) to their respective unencoded bytes.
func ReplaceEntities(b []byte, entitiesMap map[string][]byte, revEntitiesMap map[byte][]byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] == '&' && i+3 < len(b) {
			b, i = replaceEntities(b, i, entitiesMap, revEntitiesMap)
		}
	}
	return b
}

// ReplaceMultipleWhitespaceAndEntities is a combination of ReplaceMultipleWhitespace and ReplaceEntities. It is faster than executing both sequentially.
func ReplaceMultipleWhitespaceAndEntities(b []byte, entitiesMap map[string][]byte, revEntitiesMap map[byte][]byte) []byte {
	j, k := 0, 0 // j is write position, k is start of next text section
	for i := 0; i < len(b); i++ {
		if IsWhitespace(b[i]) {
			start := i
			newline := IsNewline(b[i])
			i++
			for ; i < len(b) && IsWhitespace(b[i]); i++ {
				if IsNewline(b[i]) {
					newline = true
				}
			}
			if newline {
				b[start] = '\n'
			} else {
				b[start] = ' '
			}
			if 1 < i-start { // more than one whitespace
				if j == 0 {
					j = start + 1
				} else {
					j += copy(b[j:], b[k:start+1])
				}
				k = i
			}
		}
		if i+3 < len(b) && b[i] == '&' {
			b, i = replaceEntities(b, i, entitiesMap, revEntitiesMap)
		}
	}
	if j == 0 {
		return b
	} else if j == 1 { // only if starts with whitespace
		b[k-1] = b[0]
		return b[k-1:]
	} else if k < len(b) {
		j += copy(b[j:], b[k:])
	}
	return b[:j]
}

func DecodeURL(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] == '%' && i+2 < len(b) {
			j := i + 1
			c := 0
			for ; j < i+3 && (b[j] >= '0' && b[j] <= '9' || b[j] >= 'a' && b[j] <= 'z' || b[j] >= 'A' && b[j] <= 'Z'); j++ {
				if b[j] <= '9' {
					c = c<<4 + int(b[j]-'0')
				} else if b[j] <= 'F' {
					c = c<<4 + int(b[j]-'A') + 10
				} else if b[j] <= 'f' {
					c = c<<4 + int(b[j]-'a') + 10
				}
			}
			if j == i+3 && c < 128 {
				b[i] = byte(c)
				b = append(b[:i+1], b[i+3:]...)
			}
		} else if b[i] == '+' {
			b[i] = ' '
		}
	}
	return b
}

var URLEncodingTable = [256]bool{
	// ASCII
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,

	false, false, true, true, true, true, true, false, // space, !, '
	false, false, false, true, true, false, false, true, // (, ), *, -, .
	false, false, false, false, false, false, false, false, // 0, 1, 2, 3, 4, 5, 6, 7
	false, false, true, true, true, true, true, true, // 8, 9

	true, false, false, false, false, false, false, false, // A, B, C, D, E, F, G
	false, false, false, false, false, false, false, false, // H, I, J, K, L, M, N, O
	false, false, false, false, false, false, false, false, // P, Q, R, S, T, U, V, W
	false, false, false, true, true, true, true, false, // X, Y, Z, _

	true, false, false, false, false, false, false, false, // a, b, c, d, e, f, g
	false, false, false, false, false, false, false, false, // h, i, j, k, l, m, n, o
	false, false, false, false, false, false, false, false, // p, q, r, s, t, u, v, w
	false, false, false, true, true, true, false, true, // x, y, z, ~

	// non-ASCII
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,

	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,

	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,

	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
	true, true, true, true, true, true, true, true,
}

func EncodeURL(b []byte, table [256]bool) []byte {
	for i := 0; i < len(b); i++ {
		c := b[i]
		if table[c] {
			b = append(b, 0, 0)
			copy(b[i+3:], b[i+1:])
			b[i+0] = '%'
			b[i+1] = "0123456789ABCDEF"[c>>4]
			b[i+2] = "0123456789ABCDEF"[c&15]
		} else if c == ' ' {
			b[i] = '+'
		}
	}
	return b
}
