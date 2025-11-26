// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	gomail "github.com/wneessen/go-mail"
	"github.com/wneessen/go-mail/smtp"
)

type gomailClient interface {
	Close() error
	DialAndSend(...*gomail.Msg) error
	DialToSMTPClientWithContext(context.Context) (*smtp.Client, error)
	CloseWithSMTPClient(*smtp.Client) error
	SetSMTPAuth(gomail.SMTPAuthType)
	SetSMTPAuthCustom(smtp.Auth)
}

var (
	newGomailClient     = func(host string, opts ...gomail.Option) (gomailClient, error) { return gomail.NewClient(host, opts...) }
	probeSMTPServerFunc = probeSMTPServer
)

// SMTPSender Sender SMTP mail sender
type SMTPSender struct{}

var _ Sender = &SMTPSender{}

// Send send email
func (s *SMTPSender) Send(_ string, _ []string, msg io.WriterTo) error {
	opts := setting.MailService

	mailMsg, ok := msg.(*gomail.Msg)
	if !ok {
		return fmt.Errorf("unexpected message type %T", msg)
	}

	host := opts.SMTPAddr
	protocol := opts.Protocol
	if protocol == "" {
		protocol = "smtp"
	}

	var clientOpts []gomail.Option
	if opts.EnableHelo {
		helo := opts.HeloHostname
		if helo == "" {
			var err error
			helo, err = os.Hostname()
			if err != nil {
				return fmt.Errorf("could not retrieve system hostname: %w", err)
			}
		}
		clientOpts = append(clientOpts, gomail.WithHELO(helo))
	}

	authHost := opts.SMTPAddr

	switch protocol {
	case "smtp+unix":
		host = "unix://" + opts.SMTPAddr
		clientOpts = append(clientOpts, gomail.WithTLSPolicy(gomail.NoTLS))
	case "smtps":
		port, err := parseSMTPPort(opts.SMTPPort)
		if err != nil {
			return err
		}
		tlsConfig, err := buildTLSConfig(opts)
		if err != nil {
			return err
		}
		clientOpts = append(clientOpts,
			gomail.WithPort(port),
			gomail.WithTLSConfig(tlsConfig),
			gomail.WithSSL(),
		)
	case "smtp+starttls":
		port, err := parseSMTPPort(opts.SMTPPort)
		if err != nil {
			return err
		}
		tlsConfig, err := buildTLSConfig(opts)
		if err != nil {
			return err
		}
		clientOpts = append(clientOpts,
			gomail.WithPort(port),
			gomail.WithTLSConfig(tlsConfig),
			gomail.WithTLSPolicy(gomail.TLSOpportunistic),
		)
	default:
		port, err := parseSMTPPort(opts.SMTPPort)
		if err != nil {
			return err
		}
		clientOpts = append(clientOpts,
			gomail.WithPort(port),
			gomail.WithTLSPolicy(gomail.NoTLS),
		)
	}

	if opts.User != "" {
		clientOpts = append(clientOpts,
			gomail.WithUsername(opts.User),
			gomail.WithPassword(opts.Passwd),
		)
	}

	client, err := newGomailClient(host, clientOpts...)
	if err != nil {
		return fmt.Errorf("could not create go-mail client: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Error("Closing SMTP client failed: %v", closeErr)
		}
	}()

	if opts.User != "" {
		hasAuth, authOptions, hasStartTLS, probeErr := probeSMTPServerFunc(client)
		if probeErr != nil {
			return fmt.Errorf("failed to probe SMTP capabilities: %w", probeErr)
		}
		if protocol == "smtp+starttls" && !hasStartTLS {
			log.Warn("StartTLS requested, but SMTP server does not support it; falling back to regular SMTP")
		}
		if !hasAuth {
			return fmt.Errorf("SMTP server does not support AUTH, but credentials provided")
		}

		authOptions = strings.ToUpper(authOptions)
		var selectedAuth smtp.Auth
		switch {
		case strings.Contains(authOptions, "CRAM-MD5"):
			selectedAuth = smtp.CRAMMD5Auth(opts.User, opts.Passwd)
		case strings.Contains(authOptions, "PLAIN"):
			selectedAuth = smtp.PlainAuth("", opts.User, opts.Passwd, authHost, false)
		case strings.Contains(authOptions, "LOGIN"):
			selectedAuth = LoginAuth(opts.User, opts.Passwd)
		case strings.Contains(authOptions, "NTLM"):
			selectedAuth = NtlmAuth(opts.User, opts.Passwd)
		}

		if selectedAuth != nil {
			client.SetSMTPAuthCustom(selectedAuth)
		} else if supportsAutoDiscover(authOptions) {
			client.SetSMTPAuth(gomail.SMTPAuthAutoDiscover)
		}
	}

	if err := client.DialAndSend(mailMsg); err != nil {
		return fmt.Errorf("failed to send message via SMTP: %w", err)
	}

	return nil
}

func parseSMTPPort(port string) (int, error) {
	if port == "" {
		return 0, fmt.Errorf("SMTP port is not configured")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return 0, fmt.Errorf("invalid SMTP port %q: %w", port, err)
	}
	return portNum, nil
}

func buildTLSConfig(opts *setting.Mailer) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.ForceTrustServerCert,
		ServerName:         opts.SMTPAddr,
	}
	if opts.UseClientCert {
		cert, err := tls.LoadX509KeyPair(opts.ClientCertFile, opts.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("could not load SMTP client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

func probeSMTPServer(client gomailClient) (bool, string, bool, error) {
	smtpClient, err := client.DialToSMTPClientWithContext(context.Background())
	if err != nil {
		return false, "", false, err
	}
	defer func() {
		if closeErr := client.CloseWithSMTPClient(smtpClient); closeErr != nil {
			log.Debug("Closing SMTP probe client failed: %v", closeErr)
		}
	}()

	hasStartTLS, _ := smtpClient.Extension("STARTTLS")
	hasAuth, authOptions := smtpClient.Extension("AUTH")
	return hasAuth, authOptions, hasStartTLS, nil
}

func supportsAutoDiscover(options string) bool {
	for _, mech := range []string{
		"SCRAM-SHA-256-PLUS",
		"SCRAM-SHA-256",
		"SCRAM-SHA-1-PLUS",
		"SCRAM-SHA-1",
		"XOAUTH2",
	} {
		if strings.Contains(options, mech) {
			return true
		}
	}
	return false
}
