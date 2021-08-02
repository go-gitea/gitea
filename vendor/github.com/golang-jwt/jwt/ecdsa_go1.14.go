//+build !go1.15

package jwt

import (
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"math/bits"
)

// Implements the Sign method from SigningMethod
// For this signing method, key must be an ecdsa.PrivateKey struct
func (m *SigningMethodECDSA) Sign(signingString string, key interface{}) (string, error) {
	// Get the key
	var ecdsaKey *ecdsa.PrivateKey
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		ecdsaKey = k
	default:
		return "", ErrInvalidKeyType
	}

	// Create the hasher
	if !m.Hash.Available() {
		return "", ErrHashUnavailable
	}

	hasher := m.Hash.New()
	hasher.Write([]byte(signingString))

	// Sign the string and return r, s
	if r, s, err := ecdsa.Sign(rand.Reader, ecdsaKey, hasher.Sum(nil)); err == nil {
		curveBits := ecdsaKey.Curve.Params().BitSize

		if m.CurveBits != curveBits {
			return "", ErrInvalidKey
		}

		keyBytes := curveBits / 8
		if curveBits%8 > 0 {
			keyBytes += 1
		}

		// We serialize the outputs (r and s) into big-endian byte arrays
		// padded with zeros on the left to make sure the sizes work out.
		// Output must be 2*keyBytes long.
		out := make([]byte, 2*keyBytes)
		fillBytesInt(r, out[0:keyBytes]) // r is assigned to the first half of output.
		fillBytesInt(s, out[keyBytes:])  // s is assigned to the second half of output.

		return EncodeSegment(out), nil
	} else {
		return "", err
	}
}

func fillBytesInt(x *big.Int, buf []byte) []byte {
	// Clear whole buffer. (This gets optimized into a memclr.)
	for i := range buf {
		buf[i] = 0
	}

	// This code is deeply inspired by go's own implementation but rewritten.

	// Although this function is called bits it returns words
	words := x.Bits()

	// Words are uints as per the definition of bits.Word and thus there are usually (64) /8 bytes per word
	bytesPerWord := bits.UintSize / 8

	// If our buffer is longer than the expected number of words start mid-way
	pos := len(buf) - len(words)*bytesPerWord

	// Now iterate across the words (backwards)
	for i := range words {
		// Grab the last word (Which is the biggest number)
		word := words[len(words)-1-i]

		// Now for each byte in the word
		// [abcd...] we want buf[0] = a, buf[1] = b ...

		for j := bytesPerWord; j > 0; j-- {
			d := byte(word)
			// if our position is less than 0 then panic
			if pos+j-1 >= 0 {
				// set the value of the byte to the byte
				buf[pos+j-1] = d
			} else if d != 0 {
				panic("math/big: buffer too small to fit value") // have to use the same panic string for complete compatibility
			}
			// shift the word 8 bits and reloop.
			word >>= 8
		}
		pos += bytesPerWord
	}

	return buf
}
