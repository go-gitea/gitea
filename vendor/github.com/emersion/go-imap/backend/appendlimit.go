package backend

import (
	"errors"
)

// An error that should be returned by User.CreateMessage when the message size
// is too big.
var ErrTooBig = errors.New("Message size exceeding limit")

// A backend that supports retrieving per-user message size limits.
type AppendLimitBackend interface {
	Backend

	// Get the fixed maximum message size in octets that the backend will accept
	// when creating a new message. If there is no limit, return nil.
	CreateMessageLimit() *uint32
}

// A user that supports retrieving per-user message size limits.
type AppendLimitUser interface {
	User

	// Get the fixed maximum message size in octets that the backend will accept
	// when creating a new message. If there is no limit, return nil.
	//
	// This overrides the global backend limit.
	CreateMessageLimit() *uint32
}
