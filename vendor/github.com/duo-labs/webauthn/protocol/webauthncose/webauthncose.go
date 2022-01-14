package webauthncose

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"hash"
	"math/big"

	"github.com/fxamacker/cbor/v2"
	"golang.org/x/crypto/ed25519"
)

// PublicKeyData The public key portion of a Relying Party-specific credential key pair, generated
// by an authenticator and returned to a Relying Party at registration time. We unpack this object
// using fxamacker's cbor library ("github.com/fxamacker/cbor/v2") which is why there are cbor tags
// included. The tag field values correspond to the IANA COSE keys that give their respective
// values.
// See §6.4.1.1 https://www.w3.org/TR/webauthn/#sctn-encoded-credPubKey-examples for examples of this
// COSE data.
type PublicKeyData struct {
	// Decode the results to int by default.
	_struct bool `cbor:",keyasint" json:"public_key"`
	// The type of key created. Should be OKP, EC2, or RSA.
	KeyType int64 `cbor:"1,keyasint" json:"kty"`
	// A COSEAlgorithmIdentifier for the algorithm used to derive the key signature.
	Algorithm int64 `cbor:"3,keyasint" json:"alg"`
}
type EC2PublicKeyData struct {
	PublicKeyData
	// If the key type is EC2, the curve on which we derive the signature from.
	Curve int64 `cbor:"-1,keyasint,omitempty" json:"crv"`
	// A byte string 32 bytes in length that holds the x coordinate of the key.
	XCoord []byte `cbor:"-2,keyasint,omitempty" json:"x"`
	// A byte string 32 bytes in length that holds the y coordinate of the key.
	YCoord []byte `cbor:"-3,keyasint,omitempty" json:"y"`
}

type RSAPublicKeyData struct {
	PublicKeyData
	// Represents the modulus parameter for the RSA algorithm
	Modulus []byte `cbor:"-1,keyasint,omitempty" json:"n"`
	// Represents the exponent parameter for the RSA algorithm
	Exponent []byte `cbor:"-2,keyasint,omitempty" json:"e"`
}

type OKPPublicKeyData struct {
	PublicKeyData
	Curve int64
	// A byte string that holds the x coordinate of the key.
	XCoord []byte `cbor:"-2,keyasint,omitempty" json:"x"`
}

// Verify Octet Key Pair (OKP) Public Key Signature
func (k *OKPPublicKeyData) Verify(data []byte, sig []byte) (bool, error) {
	var key ed25519.PublicKey = make([]byte, ed25519.PublicKeySize)
	copy(key, k.XCoord)
	return ed25519.Verify(key, data, sig), nil
}

// Verify Elliptic Curce Public Key Signature
func (k *EC2PublicKeyData) Verify(data []byte, sig []byte) (bool, error) {
	var curve elliptic.Curve
	switch COSEAlgorithmIdentifier(k.Algorithm) {
	case AlgES512: // IANA COSE code for ECDSA w/ SHA-512
		curve = elliptic.P521()
	case AlgES384: // IANA COSE code for ECDSA w/ SHA-384
		curve = elliptic.P384()
	case AlgES256: // IANA COSE code for ECDSA w/ SHA-256
		curve = elliptic.P256()
	default:
		return false, ErrUnsupportedAlgorithm
	}

	pubkey := &ecdsa.PublicKey{
		Curve: curve,
		X:     big.NewInt(0).SetBytes(k.XCoord),
		Y:     big.NewInt(0).SetBytes(k.YCoord),
	}

	type ECDSASignature struct {
		R, S *big.Int
	}

	e := &ECDSASignature{}
	f := HasherFromCOSEAlg(COSEAlgorithmIdentifier(k.PublicKeyData.Algorithm))
	h := f()
	h.Write(data)
	_, err := asn1.Unmarshal(sig, e)
	if err != nil {
		return false, ErrSigNotProvidedOrInvalid
	}
	return ecdsa.Verify(pubkey, h.Sum(nil), e.R, e.S), nil
}

// Verify RSA Public Key Signature
func (k *RSAPublicKeyData) Verify(data []byte, sig []byte) (bool, error) {
	pubkey := &rsa.PublicKey{
		N: big.NewInt(0).SetBytes(k.Modulus),
		E: int(uint(k.Exponent[2]) | uint(k.Exponent[1])<<8 | uint(k.Exponent[0])<<16),
	}

	f := HasherFromCOSEAlg(COSEAlgorithmIdentifier(k.PublicKeyData.Algorithm))
	h := f()
	h.Write(data)

	var hash crypto.Hash
	switch COSEAlgorithmIdentifier(k.PublicKeyData.Algorithm) {
	case AlgRS1:
		hash = crypto.SHA1
	case AlgPS256, AlgRS256:
		hash = crypto.SHA256
	case AlgPS384, AlgRS384:
		hash = crypto.SHA384
	case AlgPS512, AlgRS512:
		hash = crypto.SHA512
	default:
		return false, ErrUnsupportedAlgorithm
	}
	switch COSEAlgorithmIdentifier(k.PublicKeyData.Algorithm) {
	case AlgPS256, AlgPS384, AlgPS512:
		err := rsa.VerifyPSS(pubkey, hash, h.Sum(nil), sig, nil)
		return err == nil, err

	case AlgRS1, AlgRS256, AlgRS384, AlgRS512:
		err := rsa.VerifyPKCS1v15(pubkey, hash, h.Sum(nil), sig)
		return err == nil, err
	default:
		return false, ErrUnsupportedAlgorithm
	}
}

// Return which signature algorithm is being used from the COSE Key
func SigAlgFromCOSEAlg(coseAlg COSEAlgorithmIdentifier) SignatureAlgorithm {
	for _, details := range SignatureAlgorithmDetails {
		if details.coseAlg == coseAlg {
			return details.algo
		}
	}
	return UnknownSignatureAlgorithm
}

// Return the Hashing interface to be used for a given COSE Algorithm
func HasherFromCOSEAlg(coseAlg COSEAlgorithmIdentifier) func() hash.Hash {
	for _, details := range SignatureAlgorithmDetails {
		if details.coseAlg == coseAlg {
			return details.hasher
		}
	}
	// default to SHA256?  Why not.
	return crypto.SHA256.New
}

// Figure out what kind of COSE material was provided and create the data for the new key
func ParsePublicKey(keyBytes []byte) (interface{}, error) {
	pk := PublicKeyData{}
	cbor.Unmarshal(keyBytes, &pk)
	switch COSEKeyType(pk.KeyType) {
	case OctetKey:
		var o OKPPublicKeyData
		cbor.Unmarshal(keyBytes, &o)
		o.PublicKeyData = pk
		return o, nil
	case EllipticKey:
		var e EC2PublicKeyData
		cbor.Unmarshal(keyBytes, &e)
		e.PublicKeyData = pk
		return e, nil
	case RSAKey:
		var r RSAPublicKeyData
		cbor.Unmarshal(keyBytes, &r)
		r.PublicKeyData = pk
		return r, nil
	default:
		return nil, ErrUnsupportedKey
	}
}

// ParseFIDOPublicKey is only used when the appID extension is configured by the assertion response.
func ParseFIDOPublicKey(keyBytes []byte) (EC2PublicKeyData, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), keyBytes)

	return EC2PublicKeyData{
		PublicKeyData: PublicKeyData{
			Algorithm: int64(AlgES256),
		},
		XCoord: x.Bytes(),
		YCoord: y.Bytes(),
	}, nil
}

// COSEAlgorithmIdentifier From §5.10.5. A number identifying a cryptographic algorithm. The algorithm
// identifiers SHOULD be values registered in the IANA COSE Algorithms registry
// [https://www.w3.org/TR/webauthn/#biblio-iana-cose-algs-reg], for instance, -7 for "ES256"
//  and -257 for "RS256".
type COSEAlgorithmIdentifier int

const (
	// AlgES256 ECDSA with SHA-256
	AlgES256 COSEAlgorithmIdentifier = -7
	// AlgES384 ECDSA with SHA-384
	AlgES384 COSEAlgorithmIdentifier = -35
	// AlgES512 ECDSA with SHA-512
	AlgES512 COSEAlgorithmIdentifier = -36
	// AlgRS1 RSASSA-PKCS1-v1_5 with SHA-1
	AlgRS1 COSEAlgorithmIdentifier = -65535
	// AlgRS256 RSASSA-PKCS1-v1_5 with SHA-256
	AlgRS256 COSEAlgorithmIdentifier = -257
	// AlgRS384 RSASSA-PKCS1-v1_5 with SHA-384
	AlgRS384 COSEAlgorithmIdentifier = -258
	// AlgRS512 RSASSA-PKCS1-v1_5 with SHA-512
	AlgRS512 COSEAlgorithmIdentifier = -259
	// AlgPS256 RSASSA-PSS with SHA-256
	AlgPS256 COSEAlgorithmIdentifier = -37
	// AlgPS384 RSASSA-PSS with SHA-384
	AlgPS384 COSEAlgorithmIdentifier = -38
	// AlgPS512 RSASSA-PSS with SHA-512
	AlgPS512 COSEAlgorithmIdentifier = -39
	// AlgEdDSA EdDSA
	AlgEdDSA COSEAlgorithmIdentifier = -8
)

// The Key Type derived from the IANA COSE AuthData
type COSEKeyType int

const (
	// OctetKey is an Octet Key
	OctetKey COSEKeyType = 1
	// EllipticKey is an Elliptic Curve Public Key
	EllipticKey COSEKeyType = 2
	// RSAKey is an RSA Public Key
	RSAKey COSEKeyType = 3
)

func VerifySignature(key interface{}, data []byte, sig []byte) (bool, error) {

	switch key.(type) {
	case OKPPublicKeyData:
		o := key.(OKPPublicKeyData)
		return o.Verify(data, sig)
	case EC2PublicKeyData:
		e := key.(EC2PublicKeyData)
		return e.Verify(data, sig)
	case RSAPublicKeyData:
		r := key.(RSAPublicKeyData)
		return r.Verify(data, sig)
	default:
		return false, ErrUnsupportedKey
	}
}

func DisplayPublicKey(cpk []byte) string {
	parsedKey, err := ParsePublicKey(cpk)
	if err != nil {
		return "Cannot display key"
	}
	switch parsedKey.(type) {
	case RSAPublicKeyData:
		pKey := parsedKey.(RSAPublicKeyData)
		rKey := &rsa.PublicKey{
			N: big.NewInt(0).SetBytes(pKey.Modulus),
			E: int(uint(pKey.Exponent[2]) | uint(pKey.Exponent[1])<<8 | uint(pKey.Exponent[0])<<16),
		}
		data, err := x509.MarshalPKIXPublicKey(rKey)
		if err != nil {
			return "Cannot display key"
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: data,
		})
		return fmt.Sprintf("%s", pemBytes)
	case EC2PublicKeyData:
		pKey := parsedKey.(EC2PublicKeyData)
		var curve elliptic.Curve
		switch COSEAlgorithmIdentifier(pKey.Algorithm) {
		case AlgES256:
			curve = elliptic.P256()
		case AlgES384:
			curve = elliptic.P384()
		case AlgES512:
			curve = elliptic.P521()
		default:
			return "Cannot display key"
		}
		eKey := &ecdsa.PublicKey{
			Curve: curve,
			X:     big.NewInt(0).SetBytes(pKey.XCoord),
			Y:     big.NewInt(0).SetBytes(pKey.YCoord),
		}
		data, err := x509.MarshalPKIXPublicKey(eKey)
		if err != nil {
			return "Cannot display key"
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: data,
		})
		return fmt.Sprintf("%s", pemBytes)
	case OKPPublicKeyData:
		pKey := parsedKey.(OKPPublicKeyData)
		if len(pKey.XCoord) != ed25519.PublicKeySize {
			return "Cannot display key"
		}
		var oKey ed25519.PublicKey = make([]byte, ed25519.PublicKeySize)
		copy(oKey, pKey.XCoord)
		data, err := marshalEd25519PublicKey(oKey)
		if err != nil {
			return "Cannot display key"
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: data,
		})
		return fmt.Sprintf("%s", pemBytes)

	default:
		return "Cannot display key of this type"
	}
}

// Algorithm enumerations used for
type SignatureAlgorithm int

const (
	UnknownSignatureAlgorithm SignatureAlgorithm = iota
	MD2WithRSA
	MD5WithRSA
	SHA1WithRSA
	SHA256WithRSA
	SHA384WithRSA
	SHA512WithRSA
	DSAWithSHA1
	DSAWithSHA256
	ECDSAWithSHA1
	ECDSAWithSHA256
	ECDSAWithSHA384
	ECDSAWithSHA512
	SHA256WithRSAPSS
	SHA384WithRSAPSS
	SHA512WithRSAPSS
)

var SignatureAlgorithmDetails = []struct {
	algo    SignatureAlgorithm
	coseAlg COSEAlgorithmIdentifier
	name    string
	hasher  func() hash.Hash
}{
	{SHA1WithRSA, AlgRS1, "SHA1-RSA", crypto.SHA1.New},
	{SHA256WithRSA, AlgRS256, "SHA256-RSA", crypto.SHA256.New},
	{SHA384WithRSA, AlgRS384, "SHA384-RSA", crypto.SHA384.New},
	{SHA512WithRSA, AlgRS512, "SHA512-RSA", crypto.SHA512.New},
	{SHA256WithRSAPSS, AlgPS256, "SHA256-RSAPSS", crypto.SHA256.New},
	{SHA384WithRSAPSS, AlgPS384, "SHA384-RSAPSS", crypto.SHA384.New},
	{SHA512WithRSAPSS, AlgPS512, "SHA512-RSAPSS", crypto.SHA512.New},
	{ECDSAWithSHA256, AlgES256, "ECDSA-SHA256", crypto.SHA256.New},
	{ECDSAWithSHA384, AlgES384, "ECDSA-SHA384", crypto.SHA384.New},
	{ECDSAWithSHA512, AlgES512, "ECDSA-SHA512", crypto.SHA512.New},
	{UnknownSignatureAlgorithm, AlgEdDSA, "EdDSA", crypto.SHA512.New},
}

type Error struct {
	// Short name for the type of error that has occurred
	Type string `json:"type"`
	// Additional details about the error
	Details string `json:"error"`
	// Information to help debug the error
	DevInfo string `json:"debug"`
}

var (
	ErrUnsupportedKey = &Error{
		Type:    "invalid_key_type",
		Details: "Unsupported Public Key Type",
	}
	ErrUnsupportedAlgorithm = &Error{
		Type:    "unsupported_key_algorithm",
		Details: "Unsupported public key algorithm",
	}
	ErrSigNotProvidedOrInvalid = &Error{
		Type:    "signature_not_provided_or_invalid",
		Details: "Signature invalid or not provided",
	}
)

func (err *Error) Error() string {
	return err.Details
}

func (passedError *Error) WithDetails(details string) *Error {
	err := *passedError
	err.Details = details
	return &err
}
