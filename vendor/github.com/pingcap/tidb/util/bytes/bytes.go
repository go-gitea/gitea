package bytes

// CloneBytes returns a deep copy of slice b.
func CloneBytes(b []byte) []byte {
	return append([]byte(nil), b...)
}
