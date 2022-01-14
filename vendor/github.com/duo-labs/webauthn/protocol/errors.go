package protocol

type Error struct {
	// Short name for the type of error that has occurred
	Type string `json:"type"`
	// Additional details about the error
	Details string `json:"error"`
	// Information to help debug the error
	DevInfo string `json:"debug"`
}

var (
	ErrBadRequest = &Error{
		Type:    "invalid_request",
		Details: "Error reading the requst data",
	}
	ErrChallengeMismatch = &Error{
		Type:    "challenge_mismatch",
		Details: "Stored challenge and received challenge do not match",
	}
	ErrParsingData = &Error{
		Type:    "parse_error",
		Details: "Error parsing the authenticator response",
	}
	ErrAuthData = &Error{
		Type:    "auth_data",
		Details: "Error verifying the authenticator data",
	}
	ErrVerification = &Error{
		Type:    "verification_error",
		Details: "Error validating the authenticator response",
	}
	ErrAttestation = &Error{
		Type:    "attesation_error",
		Details: "Error validating the attestation data provided",
	}
	ErrInvalidAttestation = &Error{
		Type:    "invalid_attestation",
		Details: "Invalid attestation data",
	}
	ErrAttestationFormat = &Error{
		Type:    "invalid_attestation",
		Details: "Invalid attestation format",
	}
	ErrAttestationCertificate = &Error{
		Type:    "invalid_certificate",
		Details: "Invalid attestation certificate",
	}
	ErrAssertionSignature = &Error{
		Type:    "invalid_signature",
		Details: "Assertion Signature against auth data and client hash is not valid",
	}
	ErrUnsupportedKey = &Error{
		Type:    "invalid_key_type",
		Details: "Unsupported Public Key Type",
	}
	ErrUnsupportedAlgorithm = &Error{
		Type:    "unsupported_key_algorithm",
		Details: "Unsupported public key algorithm",
	}
	ErrNotSpecImplemented = &Error{
		Type:    "spec_unimplemented",
		Details: "This field is not yet supported by the WebAuthn spec",
	}
	ErrNotImplemented = &Error{
		Type:    "not_implemented",
		Details: "This field is not yet supported by this library",
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

func (passedError *Error) WithInfo(info string) *Error {
	err := *passedError
	err.DevInfo = info
	return &err
}
