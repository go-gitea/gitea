package protocol

// From §5.4.1 (https://www.w3.org/TR/webauthn/#dictionary-pkcredentialentity).
// PublicKeyCredentialEntity describes a user account, or a WebAuthn Relying Party,
// with which a public key credential is associated.
type CredentialEntity struct {
	// A human-palatable name for the entity. Its function depends on what the PublicKeyCredentialEntity represents:
	//
	// When inherited by PublicKeyCredentialRpEntity it is a human-palatable identifier for the Relying Party,
	// intended only for display. For example, "ACME Corporation", "Wonderful Widgets, Inc." or "ОАО Примертех".
	//
	// When inherited by PublicKeyCredentialUserEntity, it is a human-palatable identifier for a user account. It is
	// intended only for display, i.e., aiding the user in determining the difference between user accounts with similar
	// displayNames. For example, "alexm", "alex.p.mueller@example.com" or "+14255551234".
	Name string `json:"name"`
	// A serialized URL which resolves to an image associated with the entity. For example,
	// this could be a user’s avatar or a Relying Party's logo. This URL MUST be an a priori
	// authenticated URL. Authenticators MUST accept and store a 128-byte minimum length for
	// an icon member’s value. Authenticators MAY ignore an icon member’s value if its length
	// is greater than 128 bytes. The URL’s scheme MAY be "data" to avoid fetches of the URL,
	// at the cost of needing more storage.
	Icon string `json:"icon,omitempty"`
}

// From §5.4.2 (https://www.w3.org/TR/webauthn/#sctn-rp-credential-params).
// The PublicKeyCredentialRpEntity is used to supply additional
// Relying Party attributes when creating a new credential.
type RelyingPartyEntity struct {
	CredentialEntity
	// A unique identifier for the Relying Party entity, which sets the RP ID.
	ID string `json:"id"`
}

// From §5.4.3 (https://www.w3.org/TR/webauthn/#sctn-user-credential-params).
// The PublicKeyCredentialUserEntity is used to supply additional
// user account attributes when creating a new credential.
type UserEntity struct {
	CredentialEntity
	// A human-palatable name for the user account, intended only for display.
	// For example, "Alex P. Müller" or "田中 倫". The Relying Party SHOULD let
	// the user choose this, and SHOULD NOT restrict the choice more than necessary.
	DisplayName string `json:"displayName,omitempty"`
	// ID is the user handle of the user account entity. To ensure secure operation,
	// authentication and authorization decisions MUST be made on the basis of this id
	// member, not the displayName nor name members. See Section 6.1 of
	// [RFC8266](https://www.w3.org/TR/webauthn/#biblio-rfc8266).
	ID []byte `json:"id"`
}
