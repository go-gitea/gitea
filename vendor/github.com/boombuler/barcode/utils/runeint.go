package utils

// RuneToInt converts a rune between '0' and '9' to an integer between 0 and 9
// If the rune is outside of this range -1 is returned.
func RuneToInt(r rune) int {
	if r >= '0' && r <= '9' {
		return int(r - '0')
	}
	return -1
}

// IntToRune converts a digit 0 - 9 to the rune '0' - '9'. If the given int is outside
// of this range 'F' is returned!
func IntToRune(i int) rune {
	if i >= 0 && i <= 9 {
		return rune(i + '0')
	}
	return 'F'
}
