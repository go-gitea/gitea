package webauthn

import (
	p "github.com/duo-labs/webauthn/protocol"
)

type Authenticator struct {
	// The AAGUID of the authenticator. An AAGUID is defined as an array containing the globally unique
	// identifier of the authenticator model being sought.
	AAGUID []byte
	// SignCount -Upon a new login operation, the Relying Party compares the stored signature counter value
	// with the new signCount value returned in the assertion’s authenticator data. If this new
	// signCount value is less than or equal to the stored value, a cloned authenticator may
	// exist, or the authenticator may be malfunctioning.
	SignCount uint32
	// CloneWarning - This is a signal that the authenticator may be cloned, i.e. at least two copies of the
	// credential private key may exist and are being used in parallel. Relying Parties should incorporate
	// this information into their risk scoring. Whether the Relying Party updates the stored signature
	// counter value in this case, or not, or fails the authentication ceremony or not, is Relying Party-specific.
	CloneWarning bool
}

// Allow for easy marhsalling of authenticator options that are provided to the user
func SelectAuthenticator(att string, rrk *bool, uv string) p.AuthenticatorSelection {
	return p.AuthenticatorSelection{
		AuthenticatorAttachment: p.AuthenticatorAttachment(att),
		RequireResidentKey:      rrk,
		UserVerification:        p.UserVerificationRequirement(uv),
	}
}

// VerifyCounter
// Step 17 of §7.2. about verifying attestation. If the signature counter value authData.signCount
// is nonzero or the value stored in conjunction with credential’s id attribute is nonzero, then
// run the following sub-step:
//
//  If the signature counter value authData.signCount is
//
//  → Greater than the signature counter value stored in conjunction with credential’s id attribute.
//  Update the stored signature counter value, associated with credential’s id attribute, to be the value of
//  authData.signCount.
//
//  → Less than or equal to the signature counter value stored in conjunction with credential’s id attribute.
//  This is a signal that the authenticator may be cloned, see CloneWarning above for more information.
func (a *Authenticator) UpdateCounter(authDataCount uint32) {
	if authDataCount <= a.SignCount && (authDataCount != 0 || a.SignCount != 0) {
		a.CloneWarning = true
		return
	}
	a.SignCount = authDataCount
}
