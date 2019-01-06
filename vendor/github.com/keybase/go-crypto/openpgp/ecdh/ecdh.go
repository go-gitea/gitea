package ecdh

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"github.com/keybase/go-crypto/curve25519"
	"io"
	"math/big"
)

type PublicKey struct {
	elliptic.Curve
	X, Y *big.Int
}

type PrivateKey struct {
	PublicKey
	X *big.Int
}

// KDF implements Key Derivation Function as described in
// https://tools.ietf.org/html/rfc6637#section-7
func (e *PublicKey) KDF(S []byte, kdfParams []byte, hash crypto.Hash) []byte {
	sLen := (e.Curve.Params().P.BitLen() + 7) / 8
	buf := new(bytes.Buffer)
	buf.Write([]byte{0, 0, 0, 1})
	if sLen > len(S) {
		// zero-pad the S. If we got invalid S (bigger than curve's
		// P), we are going to produce invalid key. Garbage in,
		// garbage out.
		buf.Write(make([]byte, sLen-len(S)))
	}
	buf.Write(S)
	buf.Write(kdfParams)

	hashw := hash.New()

	hashw.Write(buf.Bytes())
	key := hashw.Sum(nil)

	return key
}

// AESKeyUnwrap implements RFC 3394 Key Unwrapping. See
// http://tools.ietf.org/html/rfc3394#section-2.2.1
// Note: The second described algorithm ("index-based") is implemented
// here.
func AESKeyUnwrap(key, cipherText []byte) ([]byte, error) {
	if len(cipherText)%8 != 0 {
		return nil, errors.New("cipherText must by a multiple of 64 bits")
	}

	cipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	nblocks := len(cipherText)/8 - 1

	// 1) Initialize variables.
	// - Set A = C[0]
	var A [aes.BlockSize]byte
	copy(A[:8], cipherText[:8])

	// For i = 1 to n
	//   Set R[i] = C[i]
	R := make([]byte, len(cipherText)-8)
	copy(R, cipherText[8:])

	// 2) Compute intermediate values.
	for j := 5; j >= 0; j-- {
		for i := nblocks - 1; i >= 0; i-- {
			// B = AES-1(K, (A ^ t) | R[i]) where t = n*j+i
			// A = MSB(64, B)
			t := uint64(nblocks*j + i + 1)
			At := binary.BigEndian.Uint64(A[:8]) ^ t
			binary.BigEndian.PutUint64(A[:8], At)

			copy(A[8:], R[i*8:i*8+8])
			cipher.Decrypt(A[:], A[:])

			// R[i] = LSB(B, 64)
			copy(R[i*8:i*8+8], A[8:])
		}
	}

	// 3) Output results.
	// If A is an appropriate initial value (see 2.2.3),
	for i := 0; i < 8; i++ {
		if A[i] != 0xA6 {
			return nil, errors.New("Failed to unwrap key (A is not IV)")
		}
	}

	return R, nil
}

// AESKeyWrap implements RFC 3394 Key Wrapping. See
// https://tools.ietf.org/html/rfc3394#section-2.2.2
// Note: The second described algorithm ("index-based") is implemented
// here.
func AESKeyWrap(key, plainText []byte) ([]byte, error) {
	if len(plainText)%8 != 0 {
		return nil, errors.New("plainText must be a multiple of 64 bits")
	}

	cipher, err := aes.NewCipher(key) // NewCipher checks key size
	if err != nil {
		return nil, err
	}

	nblocks := len(plainText) / 8

	// 1) Initialize variables.
	var A [aes.BlockSize]byte
	// Section 2.2.3.1 -- Initial Value
	// http://tools.ietf.org/html/rfc3394#section-2.2.3.1
	for i := 0; i < 8; i++ {
		A[i] = 0xA6
	}

	// For i = 1 to n
	//   Set R[i] = P[i]
	R := make([]byte, len(plainText))
	copy(R, plainText)

	// 2) Calculate intermediate values.
	for j := 0; j <= 5; j++ {
		for i := 0; i < nblocks; i++ {
			// B = AES(K, A | R[i])
			copy(A[8:], R[i*8:i*8+8])
			cipher.Encrypt(A[:], A[:])

			// (Assume B = A)
			// A = MSB(64, B) ^ t where t = (n*j)+1
			t := uint64(j*nblocks + i + 1)
			At := binary.BigEndian.Uint64(A[:8]) ^ t
			binary.BigEndian.PutUint64(A[:8], At)

			// R[i] = LSB(64, B)
			copy(R[i*8:i*8+8], A[8:])
		}
	}

	// 3) Output results.
	// Set C[0] = A
	// For i = 1 to n
	//   C[i] = R[i]
	return append(A[:8], R...), nil
}

// PadBuffer pads byte buffer buf to a length being multiple of
// blockLen. Additional bytes appended to the buffer have value of the
// number padded bytes. E.g. if the buffer is 3 bytes short of being
// 40 bytes total, the appended bytes will be [03, 03, 03].
func PadBuffer(buf []byte, blockLen int) []byte {
	padding := blockLen - (len(buf) % blockLen)
	if padding == 0 {
		return buf
	}

	padBuf := make([]byte, padding)
	for i := 0; i < padding; i++ {
		padBuf[i] = byte(padding)
	}

	return append(buf, padBuf...)
}

// UnpadBuffer verifies that buffer contains proper padding and
// returns buffer without the padding, or nil if the padding was
// invalid.
func UnpadBuffer(buf []byte, dataLen int) []byte {
	padding := len(buf) - dataLen
	outBuf := buf[:dataLen]

	for i := dataLen; i < len(buf); i++ {
		if buf[i] != byte(padding) {
			// Invalid padding - bail out
			return nil
		}
	}

	return outBuf
}

func (e *PublicKey) Encrypt(random io.Reader, kdfParams []byte, plain []byte, hash crypto.Hash, kdfKeySize int) (Vx *big.Int, Vy *big.Int, C []byte, err error) {
	// Vx, Vy - encryption key

	// Note for Curve 25519 - curve25519 library already does key
	// clamping in scalarMult, so we can use generic random scalar
	// generation from elliptic.
	priv, Vx, Vy, err := elliptic.GenerateKey(e.Curve, random)
	if err != nil {
		return nil, nil, nil, err
	}

	// Sx, Sy - shared secret
	Sx, _ := e.Curve.ScalarMult(e.X, e.Y, priv)

	// Encrypt the payload with KDF-ed S as the encryption key. Pass
	// the ciphertext along with V to the recipient. Recipient can
	// generate S using V and their priv key, and then KDF(S), on
	// their own, to get encryption key and decrypt the ciphertext,
	// revealing encryption key for symmetric encryption later.

	plain = PadBuffer(plain, 8)
	key := e.KDF(Sx.Bytes(), kdfParams, hash)

	// Take only as many bytes from key as the key length (the hash
	// result might be bigger)
	encrypted, err := AESKeyWrap(key[:kdfKeySize], plain)

	return Vx, Vy, encrypted, nil
}

func (e *PrivateKey) DecryptShared(X, Y *big.Int) []byte {
	Sx, _ := e.Curve.ScalarMult(X, Y, e.X.Bytes())
	return Sx.Bytes()
}

func countBits(buffer []byte) int {
	var headerLen int
	switch buffer[0] {
	case 0x4:
		headerLen = 3
	case 0x40:
		headerLen = 7
	default:
		// Unexpected header - but we can still count the bits.
		val := buffer[0]
		headerLen = 0
		for val > 0 {
			val = val / 2
			headerLen++
		}
	}

	return headerLen + (len(buffer)-1)*8
}

// elliptic.Marshal and elliptic.Unmarshal only marshals uncompressed
// 0x4 MPI types. These functions will check if the curve is cv25519,
// and if so, use 0x40 compressed type to (un)marshal. Otherwise,
// elliptic.(Un)marshal will be called.

// Marshal encodes point into either 0x4 uncompressed point form, or
// 0x40 compressed point for Curve 25519.
func Marshal(curve elliptic.Curve, x, y *big.Int) (buf []byte, bitSize int) {
	// NOTE: Read more about MPI encoding in the RFC:
	// https://tools.ietf.org/html/rfc4880#section-3.2

	// We are required to encode size in bits, counting from the most-
	// significant non-zero bit. So assuming that the buffer never
	// starts with 0x00, we only need to count bits in the first byte
	// - and in current implentation it will always be 0x4 or 0x40.

	cv, ok := curve25519.ToCurve25519(curve)
	if ok {
		buf = cv.MarshalType40(x, y)
	} else {
		buf = elliptic.Marshal(curve, x, y)
	}

	return buf, countBits(buf)
}

// Unmarshal converts point, serialized by Marshal, into x, y pair.
// For 0x40 compressed points (for Curve 25519), y will always be 0.
// It is an error if point is not on the curve, On error, x = nil.
func Unmarshal(curve elliptic.Curve, data []byte) (x, y *big.Int) {
	cv, ok := curve25519.ToCurve25519(curve)
	if ok {
		return cv.UnmarshalType40(data)
	}

	return elliptic.Unmarshal(curve, data)
}
