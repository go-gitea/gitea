// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/smtp"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	notify_service "code.gitea.io/gitea/services/notify"

	ntlmssp "github.com/Azure/go-ntlmssp"
	"github.com/jaytaylor/html2text"
	"gopkg.in/gomail.v2"
)

// Message mail body and log info
type Message struct {
	Info            string // Message information for log purpose.
	FromAddress     string
	FromDisplayName string
	To              string // Use only one recipient to prevent leaking of addresses
	ReplyTo         string
	Subject         string
	Date            time.Time
	Body            string
	Headers         map[string][]string
}

// ToMessage converts a Message to gomail.Message
func (m *Message) ToMessage() *gomail.Message {
	msg := gomail.NewMessage()
	msg.SetAddressHeader("From", m.FromAddress, m.FromDisplayName)
	msg.SetHeader("To", m.To)
	if m.ReplyTo != "" {
		msg.SetHeader("Reply-To", m.ReplyTo)
	}
	for header := range m.Headers {
		msg.SetHeader(header, m.Headers[header]...)
	}

	if setting.MailService.SubjectPrefix != "" {
		msg.SetHeader("Subject", setting.MailService.SubjectPrefix+" "+m.Subject)
	} else {
		msg.SetHeader("Subject", m.Subject)
	}
	msg.SetDateHeader("Date", m.Date)
	msg.SetHeader("X-Auto-Response-Suppress", "All")

	plainBody, err := html2text.FromString(m.Body)
	if err != nil || setting.MailService.SendAsPlainText {
		if strings.Contains(base.TruncateString(m.Body, 100), "<html>") {
			log.Warn("Mail contains HTML but configured to send as plain text.")
		}
		msg.SetBody("text/plain", plainBody)
	} else {
		msg.SetBody("text/plain", plainBody)
		msg.AddAlternative("text/html", m.Body)
	}

	if len(msg.GetHeader("Message-ID")) == 0 {
		msg.SetHeader("Message-ID", m.generateAutoMessageID())
	}

	for k, v := range setting.MailService.OverrideHeader {
		if len(msg.GetHeader(k)) != 0 {
			log.Debug("Mailer override header '%s' as per config", k)
		}
		msg.SetHeader(k, v...)
	}

	return msg
}

// SetHeader adds additional headers to a message
func (m *Message) SetHeader(field string, value ...string) {
	m.Headers[field] = value
}

func (m *Message) generateAutoMessageID() string {
	dateMs := m.Date.UnixNano() / 1e6
	h := fnv.New64()
	if len(m.To) > 0 {
		_, _ = h.Write([]byte(m.To))
	}
	_, _ = h.Write([]byte(m.Subject))
	_, _ = h.Write([]byte(m.Body))
	return fmt.Sprintf("<autogen-%d-%016x@%s>", dateMs, h.Sum64(), setting.Domain)
}

// NewMessageFrom creates new mail message object with custom From header.
func NewMessageFrom(to, fromDisplayName, fromAddress, subject, body string) *Message {
	log.Trace("NewMessageFrom (body):\n%s", body)

	return &Message{
		FromAddress:     fromAddress,
		FromDisplayName: fromDisplayName,
		To:              to,
		Subject:         subject,
		Date:            time.Now(),
		Body:            body,
		Headers:         map[string][]string{},
	}
}

// NewMessage creates new mail message object with default From header.
func NewMessage(to, subject, body string) *Message {
	return NewMessageFrom(to, setting.MailService.FromName, setting.MailService.FromEmail, subject, body)
}

type loginAuth struct {
	username, password string
}

// LoginAuth SMTP AUTH LOGIN Auth Handler
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

// Start start SMTP login auth
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

// Next next step of SMTP login auth
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unknown fromServer: %s", string(fromServer))
		}
	}
	return nil, nil
}

type ntlmAuth struct {
	username, password, domain string
	domainNeeded               bool
}

// NtlmAuth SMTP AUTH NTLM Auth Handler
func NtlmAuth(username, password string) smtp.Auth {
	user, domain, domainNeeded := ntlmssp.GetDomain(username)
	return &ntlmAuth{user, password, domain, domainNeeded}
}

// Start starts SMTP NTLM Auth
func (a *ntlmAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	negotiateMessage, err := ntlmssp.NewNegotiateMessage(a.domain, "")
	return "NTLM", negotiateMessage, err
}

// Next next step of SMTP ntlm auth
func (a *ntlmAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		if len(fromServer) == 0 {
			return nil, fmt.Errorf("ntlm ChallengeMessage is empty")
		}
		authenticateMessage, err := ntlmssp.ProcessChallenge(fromServer, a.username, a.password, a.domainNeeded)
		return authenticateMessage, err
	}
	return nil, nil
}

// Sender SMTP mail sender
type smtpSender struct{}

// Send send email
func (s *smtpSender) Send(from string, to []string, msg io.WriterTo) error {
	opts := setting.MailService

	var network string
	var address string
	if opts.Protocol == "smtp+unix" {
		network = "unix"
		address = opts.SMTPAddr
	} else {
		network = "tcp"
		address = net.JoinHostPort(opts.SMTPAddr, opts.SMTPPort)
	}

	conn, err := net.Dial(network, address)
	if err != nil {
		return fmt.Errorf("failed to establish network connection to SMTP server: %w", err)
	}
	defer conn.Close()

	var tlsconfig *tls.Config
	if opts.Protocol == "smtps" || opts.Protocol == "smtp+starttls" {
		tlsconfig = &tls.Config{
			InsecureSkipVerify: opts.ForceTrustServerCert,
			ServerName:         opts.SMTPAddr,
		}

		if opts.UseClientCert {
			cert, err := tls.LoadX509KeyPair(opts.ClientCertFile, opts.ClientKeyFile)
			if err != nil {
				return fmt.Errorf("could not load SMTP client certificate: %w", err)
			}
			tlsconfig.Certificates = []tls.Certificate{cert}
		}
	}

	if opts.Protocol == "smtps" {
		conn = tls.Client(conn, tlsconfig)
	}

	host := "localhost"
	if opts.Protocol == "smtp+unix" {
		host = opts.SMTPAddr
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("could not initiate SMTP session: %w", err)
	}

	if opts.EnableHelo {
		hostname := opts.HeloHostname
		if len(hostname) == 0 {
			hostname, err = os.Hostname()
			if err != nil {
				return fmt.Errorf("could not retrieve system hostname: %w", err)
			}
		}

		if err = client.Hello(hostname); err != nil {
			return fmt.Errorf("failed to issue HELO command: %w", err)
		}
	}

	if opts.Protocol == "smtp+starttls" {
		hasStartTLS, _ := client.Extension("STARTTLS")
		if hasStartTLS {
			if err = client.StartTLS(tlsconfig); err != nil {
				return fmt.Errorf("failed to start TLS connection: %w", err)
			}
		} else {
			log.Warn("StartTLS requested, but SMTP server does not support it; falling back to regular SMTP")
		}
	}

	canAuth, options := client.Extension("AUTH")
	if len(opts.User) > 0 {
		if !canAuth {
			return fmt.Errorf("SMTP server does not support AUTH, but credentials provided")
		}

		var auth smtp.Auth

		if strings.Contains(options, "CRAM-MD5") {
			auth = smtp.CRAMMD5Auth(opts.User, opts.Passwd)
		} else if strings.Contains(options, "PLAIN") {
			auth = smtp.PlainAuth("", opts.User, opts.Passwd, host)
		} else if strings.Contains(options, "LOGIN") {
			// Patch for AUTH LOGIN
			auth = LoginAuth(opts.User, opts.Passwd)
		} else if strings.Contains(options, "NTLM") {
			auth = NtlmAuth(opts.User, opts.Passwd)
		}

		if auth != nil {
			if err = client.Auth(auth); err != nil {
				return fmt.Errorf("failed to authenticate SMTP: %w", err)
			}
		}
	}

	if opts.OverrideEnvelopeFrom {
		if err = client.Mail(opts.EnvelopeFrom); err != nil {
			return fmt.Errorf("failed to issue MAIL command: %w", err)
		}
	} else {
		if err = client.Mail(from); err != nil {
			return fmt.Errorf("failed to issue MAIL command: %w", err)
		}
	}

	for _, rec := range to {
		if err = client.Rcpt(rec); err != nil {
			return fmt.Errorf("failed to issue RCPT command: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to issue DATA command: %w", err)
	} else if _, err = msg.WriteTo(w); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	} else if err = w.Close(); err != nil {
		return fmt.Errorf("SMTP close failed: %w", err)
	}

	return client.Quit()
}

// Sender sendmail mail sender
type sendmailSender struct{}

// Send send email
func (s *sendmailSender) Send(from string, to []string, msg io.WriterTo) error {
	var err error
	var closeError error
	var waitError error

	envelopeFrom := from
	if setting.MailService.OverrideEnvelopeFrom {
		envelopeFrom = setting.MailService.EnvelopeFrom
	}

	args := []string{"-f", envelopeFrom, "-i"}
	args = append(args, setting.MailService.SendmailArgs...)
	args = append(args, to...)
	log.Trace("Sending with: %s %v", setting.MailService.SendmailPath, args)

	desc := fmt.Sprintf("SendMail: %s %v", setting.MailService.SendmailPath, args)

	ctx, _, finished := process.GetManager().AddContextTimeout(graceful.GetManager().HammerContext(), setting.MailService.SendmailTimeout, desc)
	defer finished()

	cmd := exec.CommandContext(ctx, setting.MailService.SendmailPath, args...)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	process.SetSysProcAttribute(cmd)

	if err = cmd.Start(); err != nil {
		_ = pipe.Close()
		return err
	}

	if setting.MailService.SendmailConvertCRLF {
		buf := &strings.Builder{}
		_, err = msg.WriteTo(buf)
		if err == nil {
			_, err = strings.NewReplacer("\r\n", "\n").WriteString(pipe, buf.String())
		}
	} else {
		_, err = msg.WriteTo(pipe)
	}

	// we MUST close the pipe or sendmail will hang waiting for more of the message
	// Also we should wait on our sendmail command even if something fails
	closeError = pipe.Close()
	waitError = cmd.Wait()
	if err != nil {
		return err
	} else if closeError != nil {
		return closeError
	}
	return waitError
}

// Sender sendmail mail sender
type dummySender struct{}

// Send send email
func (s *dummySender) Send(from string, to []string, msg io.WriterTo) error {
	buf := bytes.Buffer{}
	if _, err := msg.WriteTo(&buf); err != nil {
		return err
	}
	log.Info("Mail From: %s To: %v Body: %s", from, to, buf.String())
	return nil
}

var mailQueue *queue.WorkerPoolQueue[*Message]

// Sender sender for sending mail synchronously
var Sender gomail.Sender

// NewContext start mail queue service
func NewContext(ctx context.Context) {
	// Need to check if mailQueue is nil because in during reinstall (user had installed
	// before but switched install lock off), this function will be called again
	// while mail queue is already processing tasks, and produces a race condition.
	if setting.MailService == nil || mailQueue != nil {
		return
	}

	if setting.Service.EnableNotifyMail {
		notify_service.RegisterNotifier(NewNotifier())
	}

	switch setting.MailService.Protocol {
	case "sendmail":
		Sender = &sendmailSender{}
	case "dummy":
		Sender = &dummySender{}
	default:
		Sender = &smtpSender{}
	}

	subjectTemplates, bodyTemplates = templates.Mailer(ctx)

	mailQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "mail", func(items ...*Message) []*Message {
		for _, msg := range items {
			gomailMsg := msg.ToMessage()
			log.Trace("New e-mail sending request %s: %s", gomailMsg.GetHeader("To"), msg.Info)
			if err := gomail.Send(Sender, gomailMsg); err != nil {
				log.Error("Failed to send emails %s: %s - %v", gomailMsg.GetHeader("To"), msg.Info, err)
			} else {
				log.Trace("E-mails sent %s: %s", gomailMsg.GetHeader("To"), msg.Info)
			}
		}
		return nil
	})
	if mailQueue == nil {
		log.Fatal("Unable to create mail queue")
	}
	go graceful.GetManager().RunWithCancel(mailQueue)
}

// SendAsync send emails asynchronously (make it mockable)
var SendAsync = sendAsync

func sendAsync(msgs ...*Message) {
	if setting.MailService == nil {
		log.Error("Mailer: SendAsync is being invoked but mail service hasn't been initialized")
		return
	}

	go func() {
		for _, msg := range msgs {
			_ = mailQueue.Push(msg)
		}
	}()
}
