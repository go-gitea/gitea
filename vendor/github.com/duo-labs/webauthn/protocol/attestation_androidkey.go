package protocol

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"fmt"

	"github.com/duo-labs/webauthn/protocol/webauthncose"
)

var androidAttestationKey = "android-key"

func init() {
	RegisterAttestationFormat(androidAttestationKey, verifyAndroidKeyFormat)
}

// From §8.4. https://www.w3.org/TR/webauthn/#android-key-attestation
// The android-key attestation statement looks like:
// $$attStmtType //= (
// 	fmt: "android-key",
// 	attStmt: androidStmtFormat
// )
// androidStmtFormat = {
// 		alg: COSEAlgorithmIdentifier,
// 		sig: bytes,
// 		x5c: [ credCert: bytes, * (caCert: bytes) ]
//   }
func verifyAndroidKeyFormat(att AttestationObject, clientDataHash []byte) (string, []interface{}, error) {
	// Given the verification procedure inputs attStmt, authenticatorData and clientDataHash, the verification procedure is as follows:
	// §8.4.1. Verify that attStmt is valid CBOR conforming to the syntax defined above and perform CBOR decoding on it to extract
	// the contained fields.

	// Get the alg value - A COSEAlgorithmIdentifier containing the identifier of the algorithm
	// used to generate the attestation signature.
	alg, present := att.AttStatement["alg"].(int64)
	if !present {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Error retreiving alg value")
	}

	// Get the sig value - A byte string containing the attestation signature.
	sig, present := att.AttStatement["sig"].([]byte)
	if !present {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Error retreiving sig value")
	}

	// If x5c is not present, return an error
	x5c, x509present := att.AttStatement["x5c"].([]interface{})
	if !x509present {
		// Handle Basic Attestation steps for the x509 Certificate
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Error retreiving x5c value")
	}

	// §8.4.2. Verify that sig is a valid signature over the concatenation of authenticatorData and clientDataHash
	// using the public key in the first certificate in x5c with the algorithm specified in alg.
	attCertBytes, valid := x5c[0].([]byte)
	if !valid {
		return androidAttestationKey, nil, ErrAttestation.WithDetails("Error getting certificate from x5c cert chain")
	}

	signatureData := append(att.RawAuthData, clientDataHash...)

	attCert, err := x509.ParseCertificate(attCertBytes)
	if err != nil {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails(fmt.Sprintf("Error parsing certificate from ASN.1 data: %+v", err))
	}

	coseAlg := webauthncose.COSEAlgorithmIdentifier(alg)
	sigAlg := webauthncose.SigAlgFromCOSEAlg(coseAlg)
	err = attCert.CheckSignature(x509.SignatureAlgorithm(sigAlg), signatureData, sig)
	if err != nil {
		return androidAttestationKey, nil, ErrInvalidAttestation.WithDetails(fmt.Sprintf("Signature validation error: %+v\n", err))
	}
	// Verify that the public key in the first certificate in x5c matches the credentialPublicKey in the attestedCredentialData in authenticatorData.
	pubKey, err := webauthncose.ParsePublicKey(att.AuthData.AttData.CredentialPublicKey)
	if err != nil {
		return androidAttestationKey, nil, ErrInvalidAttestation.WithDetails(fmt.Sprintf("Error parsing public key: %+v\n", err))
	}
	e := pubKey.(webauthncose.EC2PublicKeyData)
	valid, err = e.Verify(signatureData, sig)
	if err != nil || valid != true {
		return androidAttestationKey, nil, ErrInvalidAttestation.WithDetails(fmt.Sprintf("Error parsing public key: %+v\n", err))
	}
	// §8.4.3. Verify that the attestationChallenge field in the attestation certificate extension data is identical to clientDataHash.
	// attCert.Extensions
	var attExtBytes []byte
	for _, ext := range attCert.Extensions {
		if ext.Id.Equal([]int{1, 3, 6, 1, 4, 1, 11129, 2, 1, 17}) {
			attExtBytes = ext.Value
		}
	}
	if len(attExtBytes) == 0 {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Attestation certificate extensions missing 1.3.6.1.4.1.11129.2.1.17")
	}
	// As noted in §8.4.1 (https://w3c.github.io/webauthn/#key-attstn-cert-requirements) the Android Key Attestation attestation certificate's
	// android key attestation certificate extension data is identified by the OID "1.3.6.1.4.1.11129.2.1.17".
	decoded := keyDescription{}
	_, err = asn1.Unmarshal([]byte(attExtBytes), &decoded)
	if err != nil {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Unable to parse Android key attestation certificate extensions")
	}
	// Verify that the attestationChallenge field in the attestation certificate extension data is identical to clientDataHash.
	if 0 != bytes.Compare(decoded.AttestationChallenge, clientDataHash) {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Attestation challenge not equal to clientDataHash")
	}
	// The AuthorizationList.allApplications field is not present on either authorization list (softwareEnforced nor teeEnforced), since PublicKeyCredential MUST be scoped to the RP ID.
	if nil != decoded.SoftwareEnforced.AllApplications || nil != decoded.TeeEnforced.AllApplications {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Attestation certificate extensions contains all applications field")
	}
	// For the following, use only the teeEnforced authorization list if the RP wants to accept only keys from a trusted execution environment, otherwise use the union of teeEnforced and softwareEnforced.
	// The value in the AuthorizationList.origin field is equal to KM_ORIGIN_GENERATED.  (which == 0)
	if KM_ORIGIN_GENERATED != decoded.SoftwareEnforced.Origin || KM_ORIGIN_GENERATED != decoded.TeeEnforced.Origin {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Attestation certificate extensions contains authorization list with origin not equal KM_ORIGIN_GENERATED")
	}
	// The value in the AuthorizationList.purpose field is equal to KM_PURPOSE_SIGN.  (which == 2)
	if !contains(decoded.SoftwareEnforced.Purpose, KM_PURPOSE_SIGN) && !contains(decoded.TeeEnforced.Purpose, KM_PURPOSE_SIGN) {
		return androidAttestationKey, nil, ErrAttestationFormat.WithDetails("Attestation certificate extensions contains authorization list with purpose not equal KM_PURPOSE_SIGN")
	}
	return androidAttestationKey, x5c, err
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

type keyDescription struct {
	AttestationVersion       int
	AttestationSecurityLevel asn1.Enumerated
	KeymasterVersion         int
	KeymasterSecurityLevel   asn1.Enumerated
	AttestationChallenge     []byte
	UniqueID                 []byte
	SoftwareEnforced         authorizationList
	TeeEnforced              authorizationList
}

type authorizationList struct {
	Purpose                     []int       `asn1:"tag:1,explicit,set,optional"`
	Algorithm                   int         `asn1:"tag:2,explicit,optional"`
	KeySize                     int         `asn1:"tag:3,explicit,optional"`
	Digest                      []int       `asn1:"tag:5,explicit,set,optional"`
	Padding                     []int       `asn1:"tag:6,explicit,set,optional"`
	EcCurve                     int         `asn1:"tag:10,explicit,optional"`
	RsaPublicExponent           int         `asn1:"tag:200,explicit,optional"`
	RollbackResistance          interface{} `asn1:"tag:303,explicit,optional"`
	ActiveDateTime              int         `asn1:"tag:400,explicit,optional"`
	OriginationExpireDateTime   int         `asn1:"tag:401,explicit,optional"`
	UsageExpireDateTime         int         `asn1:"tag:402,explicit,optional"`
	NoAuthRequired              interface{} `asn1:"tag:503,explicit,optional"`
	UserAuthType                int         `asn1:"tag:504,explicit,optional"`
	AuthTimeout                 int         `asn1:"tag:505,explicit,optional"`
	AllowWhileOnBody            interface{} `asn1:"tag:506,explicit,optional"`
	TrustedUserPresenceRequired interface{} `asn1:"tag:507,explicit,optional"`
	TrustedConfirmationRequired interface{} `asn1:"tag:508,explicit,optional"`
	UnlockedDeviceRequired      interface{} `asn1:"tag:509,explicit,optional"`
	AllApplications             interface{} `asn1:"tag:600,explicit,optional"`
	ApplicationID               interface{} `asn1:"tag:601,explicit,optional"`
	CreationDateTime            int         `asn1:"tag:701,explicit,optional"`
	Origin                      int         `asn1:"tag:702,explicit,optional"`
	RootOfTrust                 rootOfTrust `asn1:"tag:704,explicit,optional"`
	OsVersion                   int         `asn1:"tag:705,explicit,optional"`
	OsPatchLevel                int         `asn1:"tag:706,explicit,optional"`
	AttestationApplicationID    []byte      `asn1:"tag:709,explicit,optional"`
	AttestationIDBrand          []byte      `asn1:"tag:710,explicit,optional"`
	AttestationIDDevice         []byte      `asn1:"tag:711,explicit,optional"`
	AttestationIDProduct        []byte      `asn1:"tag:712,explicit,optional"`
	AttestationIDSerial         []byte      `asn1:"tag:713,explicit,optional"`
	AttestationIDImei           []byte      `asn1:"tag:714,explicit,optional"`
	AttestationIDMeid           []byte      `asn1:"tag:715,explicit,optional"`
	AttestationIDManufacturer   []byte      `asn1:"tag:716,explicit,optional"`
	AttestationIDModel          []byte      `asn1:"tag:717,explicit,optional"`
	VendorPatchLevel            int         `asn1:"tag:718,explicit,optional"`
	BootPatchLevel              int         `asn1:"tag:719,explicit,optional"`
}

type rootOfTrust struct {
	verifiedBootKey   []byte
	deviceLocked      bool
	verifiedBootState verifiedBootState
	verifiedBootHash  []byte
}

type verifiedBootState int

const (
	Verified verifiedBootState = iota
	SelfSigned
	Unverified
	Failed
)

/**
 * The origin of a key (or pair), i.e. where it was generated.  Note that KM_TAG_ORIGIN can be found
 * in either the hardware-enforced or software-enforced list for a key, indicating whether the key
 * is hardware or software-based.  Specifically, a key with KM_ORIGIN_GENERATED in the
 * hardware-enforced list is guaranteed never to have existed outide the secure hardware.
 */
type KM_KEY_ORIGIN int

const (
	KM_ORIGIN_GENERATED = iota /* Generated in keymaster.  Should not exist outside the TEE. */
	KM_ORIGIN_DERIVED          /* Derived inside keymaster.  Likely exists off-device. */
	KM_ORIGIN_IMPORTED         /* Imported into keymaster.  Existed as cleartext in Android. */
	KM_ORIGIN_UNKNOWN          /* Keymaster did not record origin.  This value can only be seen on
	 * keys in a keymaster0 implementation.  The keymaster0 adapter uses
	 * this value to document the fact that it is unkown whether the key
	 * was generated inside or imported into keymaster. */
)

/**
 * Possible purposes of a key (or pair).
 */
type KM_PURPOSE int

const (
	KM_PURPOSE_ENCRYPT    = iota /* Usable with RSA, EC and AES keys. */
	KM_PURPOSE_DECRYPT           /* Usable with RSA, EC and AES keys. */
	KM_PURPOSE_SIGN              /* Usable with RSA, EC and HMAC keys. */
	KM_PURPOSE_VERIFY            /* Usable with RSA, EC and HMAC keys. */
	KM_PURPOSE_DERIVE_KEY        /* Usable with EC keys. */
	KM_PURPOSE_WRAP              /* Usable with wrapped keys. */
)
