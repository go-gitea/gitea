// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hcaptcha

const (
	ErrMissingInputSecret           ErrorCode = "missing-input-secret"
	ErrInvalidInputSecret           ErrorCode = "invalid-input-secret"
	ErrMissingInputResponse         ErrorCode = "missing-input-response"
	ErrInvalidInputResponse         ErrorCode = "invalid-input-response"
	ErrBadRequest                   ErrorCode = "bad-request"
	ErrInvalidOrAlreadySeenResponse ErrorCode = "invalid-or-already-seen-response"
	ErrNotUsingDummyPasscode        ErrorCode = "not-using-dummy-passcode"
	ErrSitekeySecretMismatch        ErrorCode = "sitekey-secret-mismatch"
)

// ErrorCode is any possible error from hCaptcha
type ErrorCode string

// String fulfills the Stringer interface
func (err ErrorCode) String() string {
	switch err {
	case ErrMissingInputSecret:
		return "Your secret key is missing."
	case ErrInvalidInputSecret:
		return "Your secret key is invalid or malformed."
	case ErrMissingInputResponse:
		return "The response parameter (verification token) is missing."
	case ErrInvalidInputResponse:
		return "The response parameter (verification token) is invalid or malformed."
	case ErrBadRequest:
		return "The request is invalid or malformed."
	case ErrInvalidOrAlreadySeenResponse:
		return "The response parameter has already been checked, or has another issue."
	case ErrNotUsingDummyPasscode:
		return "You have used a testing sitekey but have not used its matching secret."
	case ErrSitekeySecretMismatch:
		return "The sitekey is not registered with the provided secret."
	default:
		return ""
	}
}

// Error fulfills the error interface
func (err ErrorCode) Error() string {
	return err.String()
}
