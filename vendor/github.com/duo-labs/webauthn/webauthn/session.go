package webauthn

import "github.com/duo-labs/webauthn/protocol"

// SessionData is the data that should be stored by the Relying Party for
// the duration of the web authentication ceremony
type SessionData struct {
	Challenge            string                               `json:"challenge"`
	UserID               []byte                               `json:"user_id"`
	AllowedCredentialIDs [][]byte                             `json:"allowed_credentials,omitempty"`
	UserVerification     protocol.UserVerificationRequirement `json:"userVerification"`
	Extensions           protocol.AuthenticationExtensions    `json:"extensions,omitempty"`
}
