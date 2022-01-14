package protocol

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// From §5.2.1 (https://www.w3.org/TR/webauthn/#authenticatorattestationresponse)
// "The authenticator's response to a client’s request for the creation
// of a new public key credential. It contains information about the new credential
// that can be used to identify it for later use, and metadata that can be used by
// the WebAuthn Relying Party to assess the characteristics of the credential
// during registration."

// The initial unpacked 'response' object received by the relying party. This
// contains the clientDataJSON object, which will be marshalled into
// CollectedClientData, and the 'attestationObject', which contains
// information about the authenticator, and the newly minted
// public key credential. The information in both objects are used
// to verify the authenticity of the ceremony and new credential
type AuthenticatorAttestationResponse struct {
	// The byte slice of clientDataJSON, which becomes CollectedClientData
	AuthenticatorResponse
	// The byte slice version of AttestationObject
	// This attribute contains an attestation object, which is opaque to, and
	// cryptographically protected against tampering by, the client. The
	// attestation object contains both authenticator data and an attestation
	// statement. The former contains the AAGUID, a unique credential ID, and
	// the credential public key. The contents of the attestation statement are
	// determined by the attestation statement format used by the authenticator.
	// It also contains any additional information that the Relying Party's server
	// requires to validate the attestation statement, as well as to decode and
	// validate the authenticator data along with the JSON-serialized client data.
	AttestationObject URLEncodedBase64 `json:"attestationObject"`
}

// The parsed out version of AuthenticatorAttestationResponse.
type ParsedAttestationResponse struct {
	CollectedClientData CollectedClientData
	AttestationObject   AttestationObject
}

// From §6.4. Authenticators MUST also provide some form of attestation. The basic requirement is that the
// authenticator can produce, for each credential public key, an attestation statement verifiable by the
// WebAuthn Relying Party. Typically, this attestation statement contains a signature by an attestation
// private key over the attested credential public key and a challenge, as well as a certificate or similar
// data providing provenance information for the attestation public key, enabling the Relying Party to make
// a trust decision. However, if an attestation key pair is not available, then the authenticator MUST
// perform self attestation of the credential public key with the corresponding credential private key.
// All this information is returned by authenticators any time a new public key credential is generated, in
// the overall form of an attestation object. (https://www.w3.org/TR/webauthn/#attestation-object)
//
type AttestationObject struct {
	// The authenticator data, including the newly created public key. See AuthenticatorData for more info
	AuthData AuthenticatorData
	// The byteform version of the authenticator data, used in part for signature validation
	RawAuthData []byte `json:"authData"`
	// The format of the Attestation data.
	Format string `json:"fmt"`
	// The attestation statement data sent back if attestation is requested.
	AttStatement map[string]interface{} `json:"attStmt,omitempty"`
}

type attestationFormatValidationHandler func(AttestationObject, []byte) (string, []interface{}, error)

var attestationRegistry = make(map[string]attestationFormatValidationHandler)

// Using one of the locally registered attestation formats, handle validating the attestation
// data provided by the authenticator (and in some cases its manufacturer)
func RegisterAttestationFormat(format string, handler attestationFormatValidationHandler) {
	attestationRegistry[format] = handler
}

// Parse the values returned in the authenticator response and perform attestation verification
// Step 8. This returns a fully decoded struct with the data put into a format that can be
// used to verify the user and credential that was created
func (ccr *AuthenticatorAttestationResponse) Parse() (*ParsedAttestationResponse, error) {
	var p ParsedAttestationResponse

	err := json.Unmarshal(ccr.ClientDataJSON, &p.CollectedClientData)
	if err != nil {
		return nil, ErrParsingData.WithInfo(err.Error())
	}

	err = cbor.Unmarshal(ccr.AttestationObject, &p.AttestationObject)
	if err != nil {
		return nil, ErrParsingData.WithInfo(err.Error())
	}

	// Step 8. Perform CBOR decoding on the attestationObject field of the AuthenticatorAttestationResponse
	// structure to obtain the attestation statement format fmt, the authenticator data authData, and
	// the attestation statement attStmt.
	err = p.AttestationObject.AuthData.Unmarshal(p.AttestationObject.RawAuthData)
	if err != nil {
		return nil, fmt.Errorf("error decoding auth data: %v", err)
	}

	if !p.AttestationObject.AuthData.Flags.HasAttestedCredentialData() {
		return nil, ErrAttestationFormat.WithInfo("Attestation missing attested credential data flag")
	}

	return &p, nil
}

// Verify - Perform Steps 9 through 14 of registration verification, delegating Steps
func (attestationObject *AttestationObject) Verify(relyingPartyID string, clientDataHash []byte, verificationRequired bool) error {
	// Steps 9 through 12 are verified against the auth data.
	// These steps are identical to 11 through 14 for assertion
	// so we handle them with AuthData

	// Begin Step 9. Verify that the rpIdHash in authData is
	// the SHA-256 hash of the RP ID expected by the RP.
	rpIDHash := sha256.Sum256([]byte(relyingPartyID))
	// Handle Steps 9 through 12
	authDataVerificationError := attestationObject.AuthData.Verify(rpIDHash[:], nil, verificationRequired)
	if authDataVerificationError != nil {
		return authDataVerificationError
	}

	// Step 13. Determine the attestation statement format by performing a
	// USASCII case-sensitive match on fmt against the set of supported
	// WebAuthn Attestation Statement Format Identifier values. The up-to-date
	// list of registered WebAuthn Attestation Statement Format Identifier
	// values is maintained in the IANA registry of the same name
	// [WebAuthn-Registries] (https://www.w3.org/TR/webauthn/#biblio-webauthn-registries).

	// Since there is not an active registry yet, we'll check it against our internal
	// Supported types.

	// But first let's make sure attestation is present. If it isn't, we don't need to handle
	// any of the following steps
	if attestationObject.Format == "none" {
		if len(attestationObject.AttStatement) != 0 {
			return ErrAttestationFormat.WithInfo("Attestation format none with attestation present")
		}
		return nil
	}

	formatHandler, valid := attestationRegistry[attestationObject.Format]
	if !valid {
		return ErrAttestationFormat.WithInfo(fmt.Sprintf("Attestation format %s is unsupported", attestationObject.Format))
	}

	// Step 14. Verify that attStmt is a correct attestation statement, conveying a valid attestation signature, by using
	// the attestation statement format fmt’s verification procedure given attStmt, authData and the hash of the serialized
	// client data computed in step 7.
	attestationType, _, err := formatHandler(*attestationObject, clientDataHash)
	if err != nil {
		return err.(*Error).WithInfo(attestationType)
	}

	return nil
}
