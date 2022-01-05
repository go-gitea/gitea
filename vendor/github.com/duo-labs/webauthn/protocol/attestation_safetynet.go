package protocol

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/duo-labs/webauthn/metadata"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/mitchellh/mapstructure"
)

var safetyNetAttestationKey = "android-safetynet"

func init() {
	RegisterAttestationFormat(safetyNetAttestationKey, verifySafetyNetFormat)
}

type SafetyNetResponse struct {
	Nonce                      string        `json:"nonce"`
	TimestampMs                int64         `json:"timestampMs"`
	ApkPackageName             string        `json:"apkPackageName"`
	ApkDigestSha256            string        `json:"apkDigestSha256"`
	CtsProfileMatch            bool          `json:"ctsProfileMatch"`
	ApkCertificateDigestSha256 []interface{} `json:"apkCertificateDigestSha256"`
	BasicIntegrity             bool          `json:"basicIntegrity"`
}

// Thanks to @koesie10 and @herrjemand for outlining how to support this type really well

// §8.5. Android SafetyNet Attestation Statement Format https://w3c.github.io/webauthn/#android-safetynet-attestation
// When the authenticator in question is a platform-provided Authenticator on certain Android platforms, the attestation
// statement is based on the SafetyNet API. In this case the authenticator data is completely controlled by the caller of
// the SafetyNet API (typically an application running on the Android platform) and the attestation statement only provides
//  some statements about the health of the platform and the identity of the calling application. This attestation does not
// provide information regarding provenance of the authenticator and its associated data. Therefore platform-provided
// authenticators SHOULD make use of the Android Key Attestation when available, even if the SafetyNet API is also present.
func verifySafetyNetFormat(att AttestationObject, clientDataHash []byte) (string, []interface{}, error) {
	// The syntax of an Android Attestation statement is defined as follows:
	//     $$attStmtType //= (
	//                           fmt: "android-safetynet",
	//                           attStmt: safetynetStmtFormat
	//                       )

	//     safetynetStmtFormat = {
	//                               ver: text,
	//                               response: bytes
	//                           }

	// §8.5.1 Verify that attStmt is valid CBOR conforming to the syntax defined above and perform CBOR decoding on it to extract
	// the contained fields.

	// We have done this
	// §8.5.2 Verify that response is a valid SafetyNet response of version ver.
	version, present := att.AttStatement["ver"].(string)
	if !present {
		return safetyNetAttestationKey, nil, ErrAttestationFormat.WithDetails("Unable to find the version of SafetyNet")
	}

	if version == "" {
		return safetyNetAttestationKey, nil, ErrAttestationFormat.WithDetails("Not a proper version for SafetyNet")
	}

	// TODO: provide user the ability to designate their supported versions

	response, present := att.AttStatement["response"].([]byte)
	if !present {
		return safetyNetAttestationKey, nil, ErrAttestationFormat.WithDetails("Unable to find the SafetyNet response")
	}

	token, err := jwt.Parse(string(response), func(token *jwt.Token) (interface{}, error) {
		chain := token.Header["x5c"].([]interface{})
		o := make([]byte, base64.StdEncoding.DecodedLen(len(chain[0].(string))))
		n, err := base64.StdEncoding.Decode(o, []byte(chain[0].(string)))
		cert, err := x509.ParseCertificate(o[:n])
		return cert.PublicKey, err
	})
	if err != nil {
		return safetyNetAttestationKey, nil, ErrInvalidAttestation.WithDetails(fmt.Sprintf("Error finding cert issued to correct hostname: %+v", err))
	}

	// marshall the JWT payload into the safetynet response json
	var safetyNetResponse SafetyNetResponse
	err = mapstructure.Decode(token.Claims, &safetyNetResponse)
	if err != nil {
		return safetyNetAttestationKey, nil, ErrAttestationFormat.WithDetails(fmt.Sprintf("Error parsing the SafetyNet response: %+v", err))
	}

	// §8.5.3 Verify that the nonce in the response is identical to the Base64 encoding of the SHA-256 hash of the concatenation
	// of authenticatorData and clientDataHash.
	nonceBuffer := sha256.Sum256(append(att.RawAuthData, clientDataHash...))
	nonceBytes, err := base64.StdEncoding.DecodeString(safetyNetResponse.Nonce)
	if !bytes.Equal(nonceBuffer[:], nonceBytes) || err != nil {
		return safetyNetAttestationKey, nil, ErrInvalidAttestation.WithDetails("Invalid nonce for in SafetyNet response")
	}

	// §8.5.4 Let attestationCert be the attestation certificate (https://www.w3.org/TR/webauthn/#attestation-certificate)
	certChain := token.Header["x5c"].([]interface{})
	l := make([]byte, base64.StdEncoding.DecodedLen(len(certChain[0].(string))))
	n, err := base64.StdEncoding.Decode(l, []byte(certChain[0].(string)))
	if err != nil {
		return safetyNetAttestationKey, nil, ErrInvalidAttestation.WithDetails(fmt.Sprintf("Error finding cert issued to correct hostname: %+v", err))
	}
	attestationCert, err := x509.ParseCertificate(l[:n])
	if err != nil {
		return safetyNetAttestationKey, nil, ErrInvalidAttestation.WithDetails(fmt.Sprintf("Error finding cert issued to correct hostname: %+v", err))
	}

	// §8.5.5 Verify that attestationCert is issued to the hostname "attest.android.com"
	err = attestationCert.VerifyHostname("attest.android.com")
	if err != nil {
		return safetyNetAttestationKey, nil, ErrInvalidAttestation.WithDetails(fmt.Sprintf("Error finding cert issued to correct hostname: %+v", err))
	}

	// §8.5.6 Verify that the ctsProfileMatch attribute in the payload of response is true.
	if !safetyNetResponse.CtsProfileMatch {
		return safetyNetAttestationKey, nil, ErrInvalidAttestation.WithDetails("ctsProfileMatch attribute of the JWT payload is false")
	}

	// Verify sanity of timestamp in the payload
	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)
	t := time.Unix(safetyNetResponse.TimestampMs/1000, 0)
	if t.After(now) {
		// zero tolerance for post-dated timestamps
		return "Basic attestation with SafetyNet", nil, ErrInvalidAttestation.WithDetails("SafetyNet response with timestamp after current time")
	} else if t.Before(oneMinuteAgo) {
		// allow old timestamp for testing purposes
		// TODO: Make this user configurable
		msg := "SafetyNet response with timestamp before one minute ago"
		if metadata.Conformance {
			return "Basic attestation with SafetyNet", nil, ErrInvalidAttestation.WithDetails(msg)
		}
	}

	// §8.5.7 If successful, return implementation-specific values representing attestation type Basic and attestation
	// trust path attestationCert.
	return "Basic attestation with SafetyNet", nil, nil
}
