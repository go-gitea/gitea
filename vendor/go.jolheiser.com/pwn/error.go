package pwn

// ErrEmptyPassword is an empty password error
type ErrEmptyPassword struct{}

// Error fulfills the error interface
func (e ErrEmptyPassword) Error() string {
	return "password cannot be empty"
}

// IsErrEmptyPassword checks if an error is ErrEmptyPassword
func IsErrEmptyPassword(err error) bool {
	_, ok := err.(ErrEmptyPassword)
	return ok
}
