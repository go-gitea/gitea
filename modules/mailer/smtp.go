// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"crypto/tls"
	"net"
	"os"
	"strconv"
	"time"

	"gopkg.in/gomail.v2"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Sender SMTP mail sender
type smtpSender struct {
	queue     chan *Message
	closeChan chan struct{}
	dailer    *gomail.Dialer
}

func newSMTPSender(queue chan *Message, closeChan chan struct{}) (*smtpSender, error) {
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
		queue:     queue,
		closeChan: closeChan,
		dailer:    d,
	}

	// Start the sender routine.
	go s.processMailQueue()

	return s, err
}

func (s *smtpSender) processMailQueue() {
	var sender gomail.SendCloser
	var err error
	open := false

Loop:
	for {
		select {
		case <-s.closeChan:
			if open {
				if err = sender.Close(); err != nil {
					log.Error(3, "Failed to close mail sender connection: %v", err)
				}
			}
			return

		case msg := <-s.queue:
			log.Trace("New e-mails sending request %s: %s", msg.GetHeader("To"), msg.Info)

			// Open the smtp connection if required.
			if !open {
				if sender, err = s.dailer.Dial(); err != nil {
					log.Error(3, "Failed to send emails %s: %s - %v", msg.GetHeader("To"), msg.Info, err)
					continue Loop
				}
				open = true
			}

			if err = gomail.Send(sender, msg.Message); err != nil {
				log.Error(3, "Failed to send emails %s: %s - %v", msg.GetHeader("To"), msg.Info, err)
				continue Loop
			}

			log.Trace("E-mails sent %s: %s", msg.GetHeader("To"), msg.Info)

		// TODO: Reuse the timer.
		// Close the connection to the SMTP server if no email was sent in
		// the last 30 seconds.
		case <-time.After(30 * time.Second):
			if open {
				if err = sender.Close(); err != nil {
					log.Error(3, "Failed to close mail sender connection: %v", err)
				}
				open = false
			}
		}
	}
}
