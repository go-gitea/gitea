package ssh

import gossh "golang.org/x/crypto/ssh"

// PublicKey is an abstraction of different types of public keys.
type PublicKey interface {
	gossh.PublicKey
}

// The Permissions type holds fine-grained permissions that are specific to a
// user or a specific authentication method for a user. Permissions, except for
// "source-address", must be enforced in the server application layer, after
// successful authentication.
type Permissions struct {
	*gossh.Permissions
}

// A Signer can create signatures that verify against a public key.
type Signer interface {
	gossh.Signer
}

// ParseAuthorizedKey parses a public key from an authorized_keys file used in
// OpenSSH according to the sshd(8) manual page.
func ParseAuthorizedKey(in []byte) (out PublicKey, comment string, options []string, rest []byte, err error) {
	return gossh.ParseAuthorizedKey(in)
}

// ParsePublicKey parses an SSH public key formatted for use in
// the SSH wire protocol according to RFC 4253, section 6.6.
func ParsePublicKey(in []byte) (out PublicKey, err error) {
	return gossh.ParsePublicKey(in)
}
