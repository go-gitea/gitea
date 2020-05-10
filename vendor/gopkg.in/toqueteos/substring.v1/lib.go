package substring

// reverse is a helper fn for Suffixes
func reverse(b []byte) []byte {
	n := len(b)
	for i := 0; i < n/2; i++ {
		b[i], b[n-1-i] = b[n-1-i], b[i]
	}
	return b
}
