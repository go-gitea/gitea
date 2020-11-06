package curve25519

import (
	"crypto/elliptic"
	"math/big"
	"sync"
)

var cv25519 cv25519Curve

type cv25519Curve struct {
	*elliptic.CurveParams
}

func copyReverse(dst []byte, src []byte) {
	// Curve 25519 multiplication functions expect scalars in reverse
	// order than PGP. To keep the curve25519Curve type consistent
	// with other curves, we reverse it here.
	for i, j := 0, len(src)-1; j >= 0 && i < len(dst); i, j = i+1, j-1 {
		dst[i] = src[j]
	}
}

func copyTruncate(dst []byte, src []byte) {
	lenDst, lenSrc := len(dst), len(src)
	if lenDst == lenSrc {
		copy(dst, src)
	} else if lenDst > lenSrc {
		copy(dst[lenDst-lenSrc:lenDst], src)
	} else if lenDst < lenSrc {
		copy(dst, src[:lenDst])
	}
}

func (cv25519Curve) ScalarMult(x1, y1 *big.Int, scalar []byte) (x, y *big.Int) {
	// Assume y1 is 0 with cv25519.
	var dst [32]byte
	var x1Bytes [32]byte
	var scalarBytes [32]byte

	copyTruncate(x1Bytes[:], x1.Bytes())
	copyReverse(scalarBytes[:], scalar)

	scalarMult(&dst, &scalarBytes, &x1Bytes)

	x = new(big.Int).SetBytes(dst[:])
	y = new(big.Int)
	return x, y
}

func (cv25519Curve) ScalarBaseMult(scalar []byte) (x, y *big.Int) {
	var dst [32]byte
	var scalarBytes [32]byte
	copyReverse(scalarBytes[:], scalar[:32])
	scalarMult(&dst, &scalarBytes, &basePoint)
	x = new(big.Int).SetBytes(dst[:])
	y = new(big.Int)
	return x, y
}

func (cv25519Curve) IsOnCurve(bigX, bigY *big.Int) bool {
	return bigY.Sign() == 0 // bigY == 0 ?
}

// More information about 0x40 point format:
// https://tools.ietf.org/html/draft-koch-eddsa-for-openpgp-00#section-3
// In addition to uncompressed point format described here:
// https://tools.ietf.org/html/rfc6637#section-6

func (cv25519Curve) MarshalType40(x, y *big.Int) []byte {
	byteLen := 32

	ret := make([]byte, 1+byteLen)
	ret[0] = 0x40

	xBytes := x.Bytes()
	copyTruncate(ret[1:], xBytes)
	return ret
}

func (cv25519Curve) UnmarshalType40(data []byte) (x, y *big.Int) {
	if len(data) != 1+32 {
		return nil, nil
	}
	if data[0] != 0x40 {
		return nil, nil
	}
	x = new(big.Int).SetBytes(data[1:])
	// Any x is a valid curve point.
	return x, new(big.Int)
}

// ToCurve25519 casts given elliptic.Curve type to Curve25519 type, or
// returns nil, false if cast was unsuccessful.
func ToCurve25519(cv elliptic.Curve) (cv25519Curve, bool) {
	cv2, ok := cv.(cv25519Curve)
	return cv2, ok
}

func initCv25519() {
	cv25519.CurveParams = &elliptic.CurveParams{Name: "Curve 25519"}
	// Some code relies on these parameters being available for
	// checking Curve coordinate length. They should not be used
	// directly for any calculations.
	cv25519.P, _ = new(big.Int).SetString("7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffed", 16)
	cv25519.N, _ = new(big.Int).SetString("1000000000000000000000000000000014def9dea2f79cd65812631a5cf5d3ed", 16)
	cv25519.Gx, _ = new(big.Int).SetString("9", 16)
	cv25519.Gy, _ = new(big.Int).SetString("20ae19a1b8a086b4e01edd2c7748d14c923d4d7e6d7c61b229e9c5a27eced3d9", 16)
	cv25519.BitSize = 256
}

var initonce sync.Once

// Cv25519 returns a Curve which (partially) implements Cv25519. Only
// ScalarMult and ScalarBaseMult are valid for this curve. Add and
// Double should not be used.
func Cv25519() elliptic.Curve {
	initonce.Do(initCv25519)
	return cv25519
}

func (curve cv25519Curve) Params() *elliptic.CurveParams {
	return curve.CurveParams
}
