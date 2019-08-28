package strconv

import "math"

var float64pow10 = []float64{
	1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9,
	1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
	1e20, 1e21, 1e22,
}

// Float parses a byte-slice and returns the float it represents.
// If an invalid character is encountered, it will stop there.
func ParseFloat(b []byte) (float64, int) {
	i := 0
	neg := false
	if i < len(b) && (b[i] == '+' || b[i] == '-') {
		neg = b[i] == '-'
		i++
	}

	dot := -1
	trunk := -1
	n := uint64(0)
	for ; i < len(b); i++ {
		c := b[i]
		if c >= '0' && c <= '9' {
			if trunk == -1 {
				if n > math.MaxUint64/10 {
					trunk = i
				} else {
					n *= 10
					n += uint64(c - '0')
				}
			}
		} else if dot == -1 && c == '.' {
			dot = i
		} else {
			break
		}
	}

	f := float64(n)
	if neg {
		f = -f
	}

	mantExp := int64(0)
	if dot != -1 {
		if trunk == -1 {
			trunk = i
		}
		mantExp = int64(trunk - dot - 1)
	} else if trunk != -1 {
		mantExp = int64(trunk - i)
	}
	expExp := int64(0)
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if e, expLen := ParseInt(b[i:]); expLen > 0 {
			expExp = e
			i += expLen
		}
	}
	exp := expExp - mantExp

	// copied from strconv/atof.go
	if exp == 0 {
		return f, i
	} else if exp > 0 && exp <= 15+22 { // int * 10^k
		// If exponent is big but number of digits is not,
		// can move a few zeros into the integer part.
		if exp > 22 {
			f *= float64pow10[exp-22]
			exp = 22
		}
		if f <= 1e15 && f >= -1e15 {
			return f * float64pow10[exp], i
		}
	} else if exp < 0 && exp >= -22 { // int / 10^k
		return f / float64pow10[-exp], i
	}
	f *= math.Pow10(int(-mantExp))
	return f * math.Pow10(int(expExp)), i
}

const log2 = 0.301029995
const int64maxlen = 18

func float64exp(f float64) int {
	exp2 := 0
	if f != 0.0 {
		x := math.Float64bits(f)
		exp2 = int(x>>(64-11-1))&0x7FF - 1023 + 1
	}

	exp10 := float64(exp2) * log2
	if exp10 < 0 {
		exp10 -= 1.0
	}
	return int(exp10)
}

func AppendFloat(b []byte, f float64, prec int) ([]byte, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return b, false
	} else if prec >= int64maxlen {
		return b, false
	}

	neg := false
	if f < 0.0 {
		f = -f
		neg = true
	}
	if prec == -1 {
		prec = int64maxlen - 1
	}
	prec -= float64exp(f) // number of digits in front of the dot
	f *= math.Pow10(prec)

	// calculate mantissa and exponent
	mant := int64(f)
	mantLen := LenInt(mant)
	mantExp := mantLen - prec - 1
	if mant == 0 {
		return append(b, '0'), true
	}

	// expLen is zero for positive exponents, because positive exponents are determined later on in the big conversion loop
	exp := 0
	expLen := 0
	if mantExp > 0 {
		// positive exponent is determined in the loop below
		// but if we initially decreased the exponent to fit in an integer, we can't set the new exponent in the loop alone,
		// since the number of zeros at the end determines the positive exponent in the loop, and we just artificially lost zeros
		if prec < 0 {
			exp = mantExp
		}
		expLen = 1 + LenInt(int64(exp)) // e + digits
	} else if mantExp < -3 {
		exp = mantExp
		expLen = 2 + LenInt(int64(exp)) // e + minus + digits
	} else if mantExp < -1 {
		mantLen += -mantExp - 1 // extra zero between dot and first digit
	}

	// reserve space in b
	i := len(b)
	maxLen := 1 + mantLen + expLen // dot + mantissa digits + exponent
	if neg {
		maxLen++
	}
	if i+maxLen > cap(b) {
		b = append(b, make([]byte, maxLen)...)
	} else {
		b = b[:i+maxLen]
	}

	// write to string representation
	if neg {
		b[i] = '-'
		i++
	}

	// big conversion loop, start at the end and move to the front
	// initially print trailing zeros and remove them later on
	// for example if the first non-zero digit is three positions in front of the dot, it will overwrite the zeros with a positive exponent
	zero := true
	last := i + mantLen      // right-most position of digit that is non-zero + dot
	dot := last - prec - exp // position of dot
	j := last
	for mant > 0 {
		if j == dot {
			b[j] = '.'
			j--
		}
		newMant := mant / 10
		digit := mant - 10*newMant
		if zero && digit > 0 {
			// first non-zero digit, if we are still behind the dot we can trim the end to this position
			// otherwise trim to the dot (including the dot)
			if j > dot {
				i = j + 1
				// decrease negative exponent further to get rid of dot
				if exp < 0 {
					newExp := exp - (j - dot)
					// getting rid of the dot shouldn't lower the exponent to more digits (e.g. -9 -> -10)
					if LenInt(int64(newExp)) == LenInt(int64(exp)) {
						exp = newExp
						dot = j
						j--
						i--
					}
				}
			} else {
				i = dot
			}
			last = j
			zero = false
		}
		b[j] = '0' + byte(digit)
		j--
		mant = newMant
	}

	if j > dot {
		// extra zeros behind the dot
		for j > dot {
			b[j] = '0'
			j--
		}
		b[j] = '.'
	} else if last+3 < dot {
		// add positive exponent because we have 3 or more zeros in front of the dot
		i = last + 1
		exp = dot - last - 1
	} else if j == dot {
		// handle 0.1
		b[j] = '.'
	}

	// exponent
	if exp != 0 {
		if exp == 1 {
			b[i] = '0'
			i++
		} else if exp == 2 {
			b[i] = '0'
			b[i+1] = '0'
			i += 2
		} else {
			b[i] = 'e'
			i++
			if exp < 0 {
				b[i] = '-'
				i++
				exp = -exp
			}
			i += LenInt(int64(exp))
			j := i
			for exp > 0 {
				newExp := exp / 10
				digit := exp - 10*newExp
				j--
				b[j] = '0' + byte(digit)
				exp = newExp
			}
		}
	}
	return b[:i], true
}
