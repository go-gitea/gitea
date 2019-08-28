package strconv

// AppendPrice will append an int64 formatted as a price, where the int64 is the price in cents.
// It does not display whether a price is negative or not.
func AppendPrice(b []byte, price int64, dec bool, milSeparator byte, decSeparator byte) []byte {
	if price < 0 {
		if price == -9223372036854775808 {
			x := []byte("92 233 720 368 547 758 08")
			x[2] = milSeparator
			x[6] = milSeparator
			x[10] = milSeparator
			x[14] = milSeparator
			x[18] = milSeparator
			x[22] = decSeparator
			return append(b, x...)
		}
		price = -price
	}

	// rounding
	if !dec {
		firstDec := (price / 10) % 10
		if firstDec >= 5 {
			price += 100
		}
	}

	// calculate size
	n := LenInt(price) - 2
	if n > 0 {
		n += (n - 1) / 3 // mil separator
	} else {
		n = 1
	}
	if dec {
		n += 2 + 1 // decimals + dec separator
	}

	// resize byte slice
	i := len(b)
	if i+n > cap(b) {
		b = append(b, make([]byte, n)...)
	} else {
		b = b[:i+n]
	}

	// print fractional-part
	i += n - 1
	if dec {
		for j := 0; j < 2; j++ {
			c := byte(price%10) + '0'
			price /= 10
			b[i] = c
			i--
		}
		b[i] = decSeparator
		i--
	} else {
		price /= 100
	}

	if price == 0 {
		b[i] = '0'
		return b
	}

	// print integer-part
	j := 0
	for price > 0 {
		if j == 3 {
			b[i] = milSeparator
			i--
			j = 0
		}

		c := byte(price%10) + '0'
		price /= 10
		b[i] = c
		i--
		j++
	}
	return b
}
