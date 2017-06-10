// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

	"code.gitea.io/gitea/modules/setting"

	"gopkg.in/gomail.v2"
)

// Sender implementation for SMTP mails.
type smtpSender struct {
	mutex  sync.Mutex
	dailer *gomail.Dialer
	sender gomail.SendCloser
	isOpen bool
}

func newSMTPSender() (Sender, error) {
	opts := setting.MailService

	// Prepare the host and port.
	host, portStr, err := net.SplitHostPort(opts.Host)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	// Prepare the dailer.
	d := gomail.NewDialer(host, port, opts.User, opts.Passwd)

	if !opts.DisableHelo {
		hostname := opts.HeloHostname
		if len(hostname) == 0 {
			hostname, err = os.Hostname()
			if err != nil {
				return nil, err
			}
		}

		// LocalName is the hostname sent to the SMTP server with the HELO command.
		d.LocalName = hostname
	}

	// Prepare TLS.
	d.TLSConfig = &tls.Config{
		InsecureSkipVerify: opts.SkipVerify,
		ServerName:         host,
	}

	if opts.UseCertificate {
		cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, err
		}
		d.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	s := &smtpSender{
		dailer: d,
	}

	return s, err
}

// Send the message synchronous. The connection is opened if required.
// This method is thread-safe.
func (s *smtpSender) Send(msg *Message) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Open the smtp connection if required.
	if !s.isOpen {
		s.sender, err = s.dailer.Dial()
		if err != nil {
			return fmt.Errorf("failed to open smtp connection: %v", err)
		}
		s.isOpen = true
	}

	// Send the mail.
	err = gomail.Send(s.sender, msg.Message)
	if err != nil {
		return err
	}

	return nil
}

// Close the connection if open.
// This method is thread-safe.
func (s *smtpSender) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isOpen {
		// Always set the flag to false. Even if the sender fails to close.
		s.isOpen = false

		if err := s.sender.Close(); err != nil {
			return err
		}
	}

	return nil
}
