// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package smtp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
)

//   _________   __________________________
//  /   _____/  /     \__    ___/\______   \
//  \_____  \  /  \ /  \|    |    |     ___/
//  /        \/    Y    \    |    |    |
// /_______  /\____|__  /____|    |____|
//         \/         \/

type loginAuthenticator struct {
	username, password string
}

func (auth *loginAuthenticator) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(auth.username), nil
}

func (auth *loginAuthenticator) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(auth.username), nil
		case "Password:":
			return []byte(auth.password), nil
		}
	}
	return nil, nil
}

// SMTP authentication type names.
const (
	PlainAuthentication   = "PLAIN"
	LoginAuthentication   = "LOGIN"
	CRAMMD5Authentication = "CRAM-MD5"
)

// Authenticators contains available SMTP authentication type names.
var Authenticators = []string{PlainAuthentication, LoginAuthentication, CRAMMD5Authentication}

var (
	// ErrUnsupportedLoginType login source is unknown error
	ErrUnsupportedLoginType = errors.New("Login source is unknown")
)

// Authenticate performs an SMTP authentication.
func Authenticate(a smtp.Auth, source *Source) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: source.SkipVerify,
		ServerName:         source.Host,
	}

	conn, err := net.Dial("tcp", net.JoinHostPort(source.Host, strconv.Itoa(source.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()

	if source.UseTLS() {
		conn = tls.Client(conn, tlsConfig)
	}

	client, err := smtp.NewClient(conn, source.Host)
	if err != nil {
		return fmt.Errorf("failed to create NewClient: %w", err)
	}
	defer client.Close()

	if !source.DisableHelo {
		hostname := source.HeloHostname
		if len(hostname) == 0 {
			hostname, err = os.Hostname()
			if err != nil {
				return fmt.Errorf("failed to find Hostname: %w", err)
			}
		}

		if err = client.Hello(hostname); err != nil {
			return fmt.Errorf("failed to send Helo: %w", err)
		}
	}

	// If not using SMTPS, always use STARTTLS if available
	hasStartTLS, _ := client.Extension("STARTTLS")
	if !source.UseTLS() && hasStartTLS {
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start StartTLS: %v", err)
		}
	}

	if ok, _ := client.Extension("AUTH"); ok {
		return client.Auth(a)
	}

	return ErrUnsupportedLoginType
}
