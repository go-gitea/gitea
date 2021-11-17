package server

import (
	"crypto/tls"
	"errors"
	"net"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/commands"
	"github.com/emersion/go-sasl"
)

// IMAP errors in Not Authenticated state.
var (
	ErrAlreadyAuthenticated = errors.New("Already authenticated")
	ErrAuthDisabled         = errors.New("Authentication disabled")
)

type StartTLS struct {
	commands.StartTLS
}

func (cmd *StartTLS) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.State != imap.NotAuthenticatedState {
		return ErrAlreadyAuthenticated
	}
	if conn.IsTLS() {
		return errors.New("TLS is already enabled")
	}
	if conn.Server().TLSConfig == nil {
		return errors.New("TLS support not enabled")
	}

	// Send an OK status response to let the client know that the TLS handshake
	// can begin
	return ErrStatusResp(&imap.StatusResp{
		Type: imap.StatusRespOk,
		Info: "Begin TLS negotiation now",
	})
}

func (cmd *StartTLS) Upgrade(conn Conn) error {
	tlsConfig := conn.Server().TLSConfig

	var tlsConn *tls.Conn
	err := conn.Upgrade(func(sock net.Conn) (net.Conn, error) {
		conn.WaitReady()
		tlsConn = tls.Server(sock, tlsConfig)
		err := tlsConn.Handshake()
		return tlsConn, err
	})
	if err != nil {
		return err
	}

	conn.setTLSConn(tlsConn)

	return nil
}

func afterAuthStatus(conn Conn) error {
	caps := conn.Capabilities()
	capAtoms := make([]interface{}, 0, len(caps))
	for _, cap := range caps {
		capAtoms = append(capAtoms, imap.RawString(cap))
	}

	return ErrStatusResp(&imap.StatusResp{
		Type:      imap.StatusRespOk,
		Code:      imap.CodeCapability,
		Arguments: capAtoms,
	})
}

func canAuth(conn Conn) bool {
	for _, cap := range conn.Capabilities() {
		if cap == "AUTH=PLAIN" {
			return true
		}
	}
	return false
}

type Login struct {
	commands.Login
}

func (cmd *Login) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.State != imap.NotAuthenticatedState {
		return ErrAlreadyAuthenticated
	}
	if !canAuth(conn) {
		return ErrAuthDisabled
	}

	user, err := conn.Server().Backend.Login(conn.Info(), cmd.Username, cmd.Password)
	if err != nil {
		return err
	}

	ctx.State = imap.AuthenticatedState
	ctx.User = user
	return afterAuthStatus(conn)
}

type Authenticate struct {
	commands.Authenticate
}

func (cmd *Authenticate) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.State != imap.NotAuthenticatedState {
		return ErrAlreadyAuthenticated
	}
	if !canAuth(conn) {
		return ErrAuthDisabled
	}

	mechanisms := map[string]sasl.Server{}
	for name, newSasl := range conn.Server().auths {
		mechanisms[name] = newSasl(conn)
	}

	err := cmd.Authenticate.Handle(mechanisms, conn)
	if err != nil {
		return err
	}

	return afterAuthStatus(conn)
}
