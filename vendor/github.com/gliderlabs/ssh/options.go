package ssh

import (
	"io/ioutil"

	gossh "golang.org/x/crypto/ssh"
)

// PasswordAuth returns a functional option that sets PasswordHandler on the server.
func PasswordAuth(fn PasswordHandler) Option {
	return func(srv *Server) error {
		srv.PasswordHandler = fn
		return nil
	}
}

// PublicKeyAuth returns a functional option that sets PublicKeyHandler on the server.
func PublicKeyAuth(fn PublicKeyHandler) Option {
	return func(srv *Server) error {
		srv.PublicKeyHandler = fn
		return nil
	}
}

// HostKeyFile returns a functional option that adds HostSigners to the server
// from a PEM file at filepath.
func HostKeyFile(filepath string) Option {
	return func(srv *Server) error {
		pemBytes, err := ioutil.ReadFile(filepath)
		if err != nil {
			return err
		}

		signer, err := gossh.ParsePrivateKey(pemBytes)
		if err != nil {
			return err
		}

		srv.AddHostKey(signer)

		return nil
	}
}

// HostKeyPEM returns a functional option that adds HostSigners to the server
// from a PEM file as bytes.
func HostKeyPEM(bytes []byte) Option {
	return func(srv *Server) error {
		signer, err := gossh.ParsePrivateKey(bytes)
		if err != nil {
			return err
		}

		srv.AddHostKey(signer)

		return nil
	}
}

// NoPty returns a functional option that sets PtyCallback to return false,
// denying PTY requests.
func NoPty() Option {
	return func(srv *Server) error {
		srv.PtyCallback = func(ctx Context, pty Pty) bool {
			return false
		}
		return nil
	}
}

// WrapConn returns a functional option that sets ConnCallback on the server.
func WrapConn(fn ConnCallback) Option {
	return func(srv *Server) error {
		srv.ConnCallback = fn
		return nil
	}
}
