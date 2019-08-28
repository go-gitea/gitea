package strconv

import (
	"math"
)

// Int parses a byte-slice and returns the integer it represents.
// If an invalid character is encountered, it will stop there.
func ParseInt(b []byte) (int64, int) {
	i := 0
	neg := false
	if len(b) > 0 && (b[0] == '+' || b[0] == '-') {
		neg = b[0] == '-'
		i++
	}
	n := uint64(0)
	for i < len(b) {
		c := b[i]
		if n > math.MaxUint64/10 {
			return 0, 0
		} else if c >= '0' && c <= '9' {
			n *= 10
			n += uint64(c - '0')
		} else {
			break
		}
		i++
	}
	if !neg && n > uint64(math.MaxInt64) || n > uint64(math.MaxInt64)+1 {
		return 0, 0
	} else if neg {
		return -int64(n), i
	}
	return int64(n), i
}

func LenInt(i int64) int {
	if i < 0 {
		if i == -9223372036854775808 {
			return 19
		}
		i = -i
	}
	switch {
	case i < 10:
		return 1
	case i < 100:
		return 2
	case i < 1000:
		return 3
	case i < 10000:
		return 4
	case i < 100000:
		return 5
	case i < 1000000:
		return 6
	case i < 10000000:
		return 7
	case i < 100000000:
		return 8
	case i < 1000000000:
		return 9
	case i < 10000000000:
		return 10
	case i < 100000000000:
		return 11
	case i < 1000000000000:
		return 12
	case i < 10000000000000:
		return 13
	case i < 100000000000000:
		return 14
	case i < 1000000000000000:
		return 15
	case i < 10000000000000000:
		return 16
	case i < 100000000000000000:
		return 17
	case i < 1000000000000000000:
		return 18
	}
	return 19
}
