package googletpm

import (
	"bytes"
	"fmt"
	"math/big"
)

// DecodePublic decodes a TPMT_PUBLIC message. No error is returned if
// the input has extra trailing data.
func DecodePublic(buf []byte) (Public, error) {
	in := bytes.NewBuffer(buf)
	var pub Public
	var err error
	if err = UnpackBuf(in, &pub.Type, &pub.NameAlg, &pub.Attributes, &pub.AuthPolicy); err != nil {
		return pub, fmt.Errorf("decoding TPMT_PUBLIC: %v", err)
	}

	switch pub.Type {
	case AlgRSA:
		pub.RSAParameters, err = decodeRSAParams(in)
	case AlgECC:
		pub.ECCParameters, err = decodeECCParams(in)
	default:
		err = fmt.Errorf("unsupported type in TPMT_PUBLIC: %v", pub.Type)
	}
	return pub, err
}

// Public contains the public area of an object.
type Public struct {
	Type       Algorithm
	NameAlg    Algorithm
	Attributes KeyProp
	AuthPolicy []byte

	// If Type is AlgKeyedHash, then do not set these.
	// Otherwise, only one of the Parameters fields should be set. When encoding/decoding,
	// one will be picked based on Type.
	RSAParameters *RSAParams
	ECCParameters *ECCParams
}

// Algorithm represents a TPM_ALG_ID value.
type Algorithm uint16

// KeyProp is a bitmask used in Attributes field of key templates. Individual
// flags should be OR-ed to form a full mask.
type KeyProp uint32

// Key properties.
const (
	FlagFixedTPM            KeyProp = 0x00000002
	FlagFixedParent         KeyProp = 0x00000010
	FlagSensitiveDataOrigin KeyProp = 0x00000020
	FlagUserWithAuth        KeyProp = 0x00000040
	FlagAdminWithPolicy     KeyProp = 0x00000080
	FlagNoDA                KeyProp = 0x00000400
	FlagRestricted          KeyProp = 0x00010000
	FlagDecrypt             KeyProp = 0x00020000
	FlagSign                KeyProp = 0x00040000

	FlagSealDefault   = FlagFixedTPM | FlagFixedParent
	FlagSignerDefault = FlagSign | FlagRestricted | FlagFixedTPM |
		FlagFixedParent | FlagSensitiveDataOrigin | FlagUserWithAuth
	FlagStorageDefault = FlagDecrypt | FlagRestricted | FlagFixedTPM |
		FlagFixedParent | FlagSensitiveDataOrigin | FlagUserWithAuth
)

func decodeRSAParams(in *bytes.Buffer) (*RSAParams, error) {
	var params RSAParams
	var err error

	if params.Symmetric, err = decodeSymScheme(in); err != nil {
		return nil, fmt.Errorf("decoding Symmetric: %v", err)
	}
	if params.Sign, err = decodeSigScheme(in); err != nil {
		return nil, fmt.Errorf("decoding Sign: %v", err)
	}
	var modBytes []byte
	if err := UnpackBuf(in, &params.KeyBits, &params.Exponent, &modBytes); err != nil {
		return nil, fmt.Errorf("decoding KeyBits, Exponent, Modulus: %v", err)
	}
	if params.Exponent == 0 {
		params.encodeDefaultExponentAsZero = true
		params.Exponent = defaultRSAExponent
	}
	params.Modulus = new(big.Int).SetBytes(modBytes)
	return &params, nil
}

const defaultRSAExponent = 1<<16 + 1

// RSAParams represents parameters of an RSA key pair.
//
// Symmetric and Sign may be nil, depending on key Attributes in Public.
//
// One of Modulus and ModulusRaw must always be non-nil. Modulus takes
// precedence. ModulusRaw is used for key templates where the field named
// "unique" must be a byte array of all zeroes.
type RSAParams struct {
	Symmetric *SymScheme
	Sign      *SigScheme
	KeyBits   uint16
	// The default Exponent (65537) has two representations; the
	// 0 value, and the value 65537.
	// If encodeDefaultExponentAsZero is set, an exponent of 65537
	// will be encoded as zero. This is necessary to produce an identical
	// encoded bitstream, so Name digest calculations will be correct.
	encodeDefaultExponentAsZero bool
	Exponent                    uint32
	ModulusRaw                  []byte
	Modulus                     *big.Int
}

// SymScheme represents a symmetric encryption scheme.
type SymScheme struct {
	Alg     Algorithm
	KeyBits uint16
	Mode    Algorithm
} // SigScheme represents a signing scheme.
type SigScheme struct {
	Alg   Algorithm
	Hash  Algorithm
	Count uint32
}

func decodeSigScheme(in *bytes.Buffer) (*SigScheme, error) {
	var scheme SigScheme
	if err := UnpackBuf(in, &scheme.Alg); err != nil {
		return nil, fmt.Errorf("decoding Alg: %v", err)
	}
	if scheme.Alg == AlgNull {
		return nil, nil
	}
	if err := UnpackBuf(in, &scheme.Hash); err != nil {
		return nil, fmt.Errorf("decoding Hash: %v", err)
	}
	if scheme.Alg.UsesCount() {
		if err := UnpackBuf(in, &scheme.Count); err != nil {
			return nil, fmt.Errorf("decoding Count: %v", err)
		}
	}
	return &scheme, nil
}

// UsesCount returns true if a signature algorithm uses count value.
func (a Algorithm) UsesCount() bool {
	return a == AlgECDAA
}

func decodeKDFScheme(in *bytes.Buffer) (*KDFScheme, error) {
	var scheme KDFScheme
	if err := UnpackBuf(in, &scheme.Alg); err != nil {
		return nil, fmt.Errorf("decoding Alg: %v", err)
	}
	if scheme.Alg == AlgNull {
		return nil, nil
	}
	if err := UnpackBuf(in, &scheme.Hash); err != nil {
		return nil, fmt.Errorf("decoding Hash: %v", err)
	}
	return &scheme, nil
}
func decodeSymScheme(in *bytes.Buffer) (*SymScheme, error) {
	var scheme SymScheme
	if err := UnpackBuf(in, &scheme.Alg); err != nil {
		return nil, fmt.Errorf("decoding Alg: %v", err)
	}
	if scheme.Alg == AlgNull {
		return nil, nil
	}
	if err := UnpackBuf(in, &scheme.KeyBits, &scheme.Mode); err != nil {
		return nil, fmt.Errorf("decoding KeyBits, Mode: %v", err)
	}
	return &scheme, nil
}
func decodeECCParams(in *bytes.Buffer) (*ECCParams, error) {
	var params ECCParams
	var err error

	if params.Symmetric, err = decodeSymScheme(in); err != nil {
		return nil, fmt.Errorf("decoding Symmetric: %v", err)
	}
	if params.Sign, err = decodeSigScheme(in); err != nil {
		return nil, fmt.Errorf("decoding Sign: %v", err)
	}
	if err := UnpackBuf(in, &params.CurveID); err != nil {
		return nil, fmt.Errorf("decoding CurveID: %v", err)
	}
	if params.KDF, err = decodeKDFScheme(in); err != nil {
		return nil, fmt.Errorf("decoding KDF: %v", err)
	}
	var x, y []byte
	if err := UnpackBuf(in, &x, &y); err != nil {
		return nil, fmt.Errorf("decoding Point: %v", err)
	}
	params.Point.X = new(big.Int).SetBytes(x)
	params.Point.Y = new(big.Int).SetBytes(y)
	return &params, nil
}

// ECCParams represents parameters of an ECC key pair.
//
// Symmetric, Sign and KDF may be nil, depending on key Attributes in Public.
type ECCParams struct {
	Symmetric *SymScheme
	Sign      *SigScheme
	CurveID   EllipticCurve
	KDF       *KDFScheme
	Point     ECPoint
}

// EllipticCurve identifies specific EC curves.
type EllipticCurve uint16

// ECC curves supported by TPM 2.0 spec.
const (
	CurveNISTP192 = EllipticCurve(iota + 1)
	CurveNISTP224
	CurveNISTP256
	CurveNISTP384
	CurveNISTP521

	CurveBNP256 = EllipticCurve(iota + 10)
	CurveBNP638

	CurveSM2P256 = EllipticCurve(0x0020)
)

// ECPoint represents a ECC coordinates for a point.
type ECPoint struct {
	X, Y *big.Int
}

// KDFScheme represents a KDF (Key Derivation Function) scheme.
type KDFScheme struct {
	Alg  Algorithm
	Hash Algorithm
}
