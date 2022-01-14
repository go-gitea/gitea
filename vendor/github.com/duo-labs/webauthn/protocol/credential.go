package protocol

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
)

// The basic credential type that is inherited by WebAuthn's
// PublicKeyCredential type
// https://w3c.github.io/webappsec-credential-management/#credential
type Credential struct {
	// ID is The credential’s identifier. The requirements for the
	// identifier are distinct for each type of credential. It might
	// represent a username for username/password tuples, for example.
	ID string `json:"id"`
	// Type is the value of the object’s interface object's [[type]] slot,
	// which specifies the credential type represented by this object.
	// This should be type "public-key" for Webauthn credentials.
	Type string `json:"type"`
}

// The PublicKeyCredential interface inherits from Credential, and contains
//  the attributes that are returned to the caller when a new credential
// is created, or a new assertion is requested.
type ParsedCredential struct {
	ID   string `cbor:"id"`
	Type string `cbor:"type"`
}

type PublicKeyCredential struct {
	Credential
	RawID                  URLEncodedBase64                      `json:"rawId"`
	ClientExtensionResults AuthenticationExtensionsClientOutputs `json:"clientExtensionResults,omitempty"`
}

type ParsedPublicKeyCredential struct {
	ParsedCredential
	RawID                  []byte                                `json:"rawId"`
	ClientExtensionResults AuthenticationExtensionsClientOutputs `json:"clientExtensionResults,omitempty"`
}

type CredentialCreationResponse struct {
	PublicKeyCredential
	AttestationResponse AuthenticatorAttestationResponse `json:"response"`
}

type ParsedCredentialCreationData struct {
	ParsedPublicKeyCredential
	Response ParsedAttestationResponse
	Raw      CredentialCreationResponse
}

func ParseCredentialCreationResponse(response *http.Request) (*ParsedCredentialCreationData, error) {
	if response == nil || response.Body == nil {
		return nil, ErrBadRequest.WithDetails("No response given")
	}
	return ParseCredentialCreationResponseBody(response.Body)
}

func ParseCredentialCreationResponseBody(body io.Reader) (*ParsedCredentialCreationData, error) {
	var ccr CredentialCreationResponse
	err := json.NewDecoder(body).Decode(&ccr)
	if err != nil {
		return nil, ErrBadRequest.WithDetails("Parse error for Registration").WithInfo(err.Error())
	}

	if ccr.ID == "" {
		return nil, ErrBadRequest.WithDetails("Parse error for Registration").WithInfo("Missing ID")
	}

	testB64, err := base64.RawURLEncoding.DecodeString(ccr.ID)
	if err != nil || !(len(testB64) > 0) {
		return nil, ErrBadRequest.WithDetails("Parse error for Registration").WithInfo("ID not base64.RawURLEncoded")
	}

	if ccr.PublicKeyCredential.Credential.Type == "" {
		return nil, ErrBadRequest.WithDetails("Parse error for Registration").WithInfo("Missing type")
	}

	if ccr.PublicKeyCredential.Credential.Type != "public-key" {
		return nil, ErrBadRequest.WithDetails("Parse error for Registration").WithInfo("Type not public-key")
	}

	var pcc ParsedCredentialCreationData
	pcc.ID, pcc.RawID, pcc.Type = ccr.ID, ccr.RawID, ccr.Type
	pcc.Raw = ccr

	parsedAttestationResponse, err := ccr.AttestationResponse.Parse()
	if err != nil {
		return nil, ErrParsingData.WithDetails("Error parsing attestation response")
	}

	pcc.Response = *parsedAttestationResponse

	return &pcc, nil
}

// Verifies the Client and Attestation data as laid out by §7.1. Registering a new credential
// https://www.w3.org/TR/webauthn/#registering-a-new-credential
func (pcc *ParsedCredentialCreationData) Verify(storedChallenge string, verifyUser bool, relyingPartyID, relyingPartyOrigin string) error {

	// Handles steps 3 through 6 - Verifying the Client Data against the Relying Party's stored data
	verifyError := pcc.Response.CollectedClientData.Verify(storedChallenge, CreateCeremony, relyingPartyOrigin)
	if verifyError != nil {
		return verifyError
	}

	// Step 7. Compute the hash of response.clientDataJSON using SHA-256.
	clientDataHash := sha256.Sum256(pcc.Raw.AttestationResponse.ClientDataJSON)

	// Step 8. Perform CBOR decoding on the attestationObject field of the AuthenticatorAttestationResponse
	// structure to obtain the attestation statement format fmt, the authenticator data authData, and the
	// attestation statement attStmt. is handled while

	// We do the above step while parsing and decoding the CredentialCreationResponse
	// Handle steps 9 through 14 - This verifies the attestaion object and
	verifyError = pcc.Response.AttestationObject.Verify(relyingPartyID, clientDataHash[:], verifyUser)
	if verifyError != nil {
		return verifyError
	}

	// Step 15. If validation is successful, obtain a list of acceptable trust anchors (attestation root
	// certificates or ECDAA-Issuer public keys) for that attestation type and attestation statement
	// format fmt, from a trusted source or from policy. For example, the FIDO Metadata Service provides
	// one way to obtain such information, using the aaguid in the attestedCredentialData in authData.
	// [https://fidoalliance.org/specs/fido-v2.0-id-20180227/fido-metadata-service-v2.0-id-20180227.html]

	// TODO: There are no valid AAGUIDs yet or trust sources supported. We could implement policy for the RP in
	// the future, however.

	// Step 16. Assess the attestation trustworthiness using outputs of the verification procedure in step 14, as follows:
	// - If self attestation was used, check if self attestation is acceptable under Relying Party policy.
	// - If ECDAA was used, verify that the identifier of the ECDAA-Issuer public key used is included in
	//   the set of acceptable trust anchors obtained in step 15.
	// - Otherwise, use the X.509 certificates returned by the verification procedure to verify that the
	//   attestation public key correctly chains up to an acceptable root certificate.

	// TODO: We're not supporting trust anchors, self-attestation policy, or acceptable root certs yet

	// Step 17. Check that the credentialId is not yet registered to any other user. If registration is
	// requested for a credential that is already registered to a different user, the Relying Party SHOULD
	// fail this registration ceremony, or it MAY decide to accept the registration, e.g. while deleting
	// the older registration.

	// TODO: We can't support this in the code's current form, the Relying Party would need to check for this
	// against their database

	// Step 18 If the attestation statement attStmt verified successfully and is found to be trustworthy, then
	// register the new credential with the account that was denoted in the options.user passed to create(), by
	// associating it with the credentialId and credentialPublicKey in the attestedCredentialData in authData, as
	// appropriate for the Relying Party's system.

	// Step 19. If the attestation statement attStmt successfully verified but is not trustworthy per step 16 above,
	// the Relying Party SHOULD fail the registration ceremony.

	// TODO: Not implemented for the reasons mentioned under Step 16

	return nil
}

// GetAppID takes a AuthenticationExtensions object or nil. It then performs the following checks in order:
//
// 1. Check that the Session Data's AuthenticationExtensions has been provided and return a blank appid if it hasn't been.
// 2. Check that the AuthenticationExtensionsClientOutputs contains the extensions output and return a blank appid if it doesn't.
// 3. Check that the Credential AttestationType is `fido-u2f` and return a blank appid if it isn't.
// 4. Check that the AuthenticationExtensionsClientOutputs contains the appid key and return a blank appid if it doesn't.
// 5. Check that the AuthenticationExtensionsClientOutputs appid is a bool and return an error if it isn't.
// 6. Check that the appid output is true and return a blank appid if it isn't.
// 7. Check that the Session Data has an appid extension defined and return an error if it doesn't.
// 8. Check that the appid extension in Session Data is a string and return an error if it isn't.
// 9. Return the appid extension value from the Session Data.
func (ppkc ParsedPublicKeyCredential) GetAppID(authExt AuthenticationExtensions, credentialAttestationType string) (appID string, err error) {
	var (
		value, clientValue interface{}
		enableAppID, ok    bool
	)

	if authExt == nil {
		return "", nil
	}

	if ppkc.ClientExtensionResults == nil {
		return "", nil
	}

	// If the credential does not have the correct attestation type it is assumed to NOT be a fido-u2f credential.
	// https://w3c.github.io/webauthn/#sctn-fido-u2f-attestation
	if credentialAttestationType != "fido-u2f" {
		return "", nil
	}

	if clientValue, ok = ppkc.ClientExtensionResults["appid"]; !ok {
		return "", nil
	}

	if enableAppID, ok = clientValue.(bool); !ok {
		return "", ErrBadRequest.WithDetails("Client Output appid did not have the expected type")
	}

	if !enableAppID {
		return "", nil
	}

	if value, ok = authExt["appid"]; !ok {
		return "", ErrBadRequest.WithDetails("Session Data does not have an appid but Client Output indicates it should be set")
	}

	if appID, ok = value.(string); !ok {
		return "", ErrBadRequest.WithDetails("Session Data appid did not have the expected type")
	}

	return appID, nil
}
