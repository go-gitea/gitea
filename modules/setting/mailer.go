// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net"
	"net/mail"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"

	shellquote "github.com/kballard/go-shellquote"
)

// Mailer represents mail service.
type Mailer struct {
	// Mailer
	Name                 string
	From                 string
	EnvelopeFrom         string
	OverrideEnvelopeFrom bool `ini:"-"`
	FromName             string
	FromEmail            string
	SendAsPlainText      bool
	SubjectPrefix        string

	// SMTP sender
	Protocol             string
	SMTPAddr             string
	SMTPPort             string
	User, Passwd         string
	EnableHelo           bool
	HeloHostname         string
	ForceTrustServerCert bool
	UseClientCert        bool
	ClientCertFile       string
	ClientKeyFile        string

	// Sendmail sender
	SendmailPath        string
	SendmailArgs        []string
	SendmailTimeout     time.Duration
	SendmailConvertCRLF bool
}

// MailService the global mailer
var MailService *Mailer

func newMailService() {
	sec := Cfg.Section("mailer")
	// Check mailer setting.
	if !sec.Key("ENABLED").MustBool() {
		return
	}

	MailService = &Mailer{
		Name:            sec.Key("NAME").MustString(AppName),
		SendAsPlainText: sec.Key("SEND_AS_PLAIN_TEXT").MustBool(false),

		Protocol:             sec.Key("PROTOCOL").In("", []string{"smtp", "smtps", "smtp+startls", "smtp+unix", "sendmail", "dummy"}),
		SMTPAddr:             sec.Key("SMTP_ADDR").String(),
		SMTPPort:             sec.Key("SMTP_PORT").String(),
		User:                 sec.Key("USER").String(),
		Passwd:               sec.Key("PASSWD").String(),
		EnableHelo:           sec.Key("ENABLE_HELO").MustBool(true),
		HeloHostname:         sec.Key("HELO_HOSTNAME").String(),
		ForceTrustServerCert: sec.Key("FORCE_TRUST_SERVER_CERT").MustBool(false),
		UseClientCert:        sec.Key("USE_CLIENT_CERT").MustBool(false),
		ClientCertFile:       sec.Key("CLIENT_CERT_FILE").String(),
		ClientKeyFile:        sec.Key("CLIENT_KEY_FILE").String(),
		SubjectPrefix:        sec.Key("SUBJECT_PREFIX").MustString(""),

		SendmailPath:        sec.Key("SENDMAIL_PATH").MustString("sendmail"),
		SendmailTimeout:     sec.Key("SENDMAIL_TIMEOUT").MustDuration(5 * time.Minute),
		SendmailConvertCRLF: sec.Key("SENDMAIL_CONVERT_CRLF").MustBool(true),
	}
	MailService.From = sec.Key("FROM").MustString(MailService.User)
	MailService.EnvelopeFrom = sec.Key("ENVELOPE_FROM").MustString("")

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "MAILER_TYPE", "mailer", "PROTOCOL")
	if sec.HasKey("MAILER_TYPE") && !sec.HasKey("PROTOCOL") {
		if sec.Key("MAILER_TYPE").String() == "sendmail" {
			MailService.Protocol = "sendmail"
		}
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "HOST", "mailer", "SMTP_ADDR")
	if sec.HasKey("HOST") && !sec.HasKey("SMTP_ADDR") {
		givenHost := sec.Key("HOST").String()
		addr, port, err := net.SplitHostPort(givenHost)
		if err != nil {
			log.Fatal("Invalid mailer.HOST (%s): %v", givenHost, err)
		}
		MailService.SMTPAddr = addr
		MailService.SMTPPort = port
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "IS_TLS_ENABLED", "mailer", "PROTOCOL")
	if sec.HasKey("IS_TLS_ENABLED") && !sec.HasKey("PROTOCOL") {
		if sec.Key("IS_TLS_ENABLED").MustBool() {
			MailService.Protocol = "smtps"
		} else {
			MailService.Protocol = "smtp+startls"
		}
	}

	if MailService.SMTPPort == "" {
		switch MailService.Protocol {
		case "smtp":
			MailService.SMTPPort = "25"
		case "smtps":
			MailService.SMTPPort = "465"
		case "smtp+startls":
			MailService.SMTPPort = "587"
		}
	}

	if MailService.Protocol == "" {
		if strings.ContainsAny(MailService.SMTPAddr, "/\\") {
			MailService.Protocol = "smtp+unix"
		} else {
			switch MailService.SMTPPort {
			case "25":
				MailService.Protocol = "smtp"
			case "465":
				MailService.Protocol = "smtps"
			case "587":
				MailService.Protocol = "smtp+startls"
			default:
				log.Error("unable to infer unspecified mailer.PROTOCOL from mailer.SMTP_PORT = %q, assume using smtps", MailService.SMTPPort)
				MailService.Protocol = "smtps"
			}
		}
	}

	// we want to warn if users use SMTP on a non-local IP;
	// we might as well take the opportunity to check that it has an IP at all
	ips := tryResolveAddr(MailService.SMTPAddr)
	if MailService.Protocol == "smtp" {
		for _, ip := range ips {
			if !ip.IsLoopback() {
				log.Warn("connecting over insecure SMTP protocol to non-local address is not recommended")
				break
			}
		}
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "DISABLE_HELO", "mailer", "ENABLE_HELO")
	if sec.HasKey("DISABLE_HELO") && !sec.HasKey("ENABLE_HELO") {
		MailService.EnableHelo = !sec.Key("DISABLE_HELO").MustBool()
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "SKIP_VERIFY", "mailer", "FORCE_TRUST_SERVER_CERT")
	if sec.HasKey("SKIP_VERIFY") && !sec.HasKey("FORCE_TRUST_SERVER_CERT") {
		MailService.ForceTrustServerCert = sec.Key("SKIP_VERIFY").MustBool()
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "USE_CERTIFICATE", "mailer", "USE_CLIENT_CERT")
	if sec.HasKey("USE_CERTIFICATE") && !sec.HasKey("USE_CLIENT_CERT") {
		MailService.UseClientCert = sec.Key("USE_CLIENT_CERT").MustBool()
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "CERT_FILE", "mailer", "CLIENT_CERT_FILE")
	if sec.HasKey("CERT_FILE") && !sec.HasKey("CLIENT_CERT_FILE") {
		MailService.ClientCertFile = sec.Key("CERT_FILE").String()
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "KEY_FILE", "mailer", "CLIENT_KEY_FILE")
	if sec.HasKey("KEY_FILE") && !sec.HasKey("CLIENT_KEY_FILE") {
		MailService.ClientKeyFile = sec.Key("KEY_FILE").String()
	}

	// FIXME: DEPRECATED to be removed in v1.19.0
	deprecatedSetting("mailer", "ENABLE_HTML_ALTERNATIVE", "mailer", "SEND_AS_PLAIN_TEXT")
	if sec.HasKey("ENABLE_HTML_ALTERNATIVE") && !sec.HasKey("SEND_AS_PLAIN_TEXT") {
		MailService.SendAsPlainText = !sec.Key("ENABLE_HTML_ALTERNATIVE").MustBool(false)
	}

	if MailService.From != "" {
		parsed, err := mail.ParseAddress(MailService.From)
		if err != nil {
			log.Fatal("Invalid mailer.FROM (%s): %v", MailService.From, err)
		}
		MailService.FromName = parsed.Name
		MailService.FromEmail = parsed.Address
	} else {
		log.Error("no mailer.FROM provided, email system may not work.")
	}

	switch MailService.EnvelopeFrom {
	case "":
		MailService.OverrideEnvelopeFrom = false
	case "<>":
		MailService.EnvelopeFrom = ""
		MailService.OverrideEnvelopeFrom = true
	default:
		parsed, err := mail.ParseAddress(MailService.EnvelopeFrom)
		if err != nil {
			log.Fatal("Invalid mailer.ENVELOPE_FROM (%s): %v", MailService.EnvelopeFrom, err)
		}
		MailService.OverrideEnvelopeFrom = true
		MailService.EnvelopeFrom = parsed.Address
	}

	if MailService.Protocol == "sendmail" {
		var err error
		MailService.SendmailArgs, err = shellquote.Split(sec.Key("SENDMAIL_ARGS").String())
		if err != nil {
			log.Error("Failed to parse Sendmail args: %s with error %v", CustomConf, err)
		}
	}

	log.Info("Mail Service Enabled")
}

func newRegisterMailService() {
	if !Cfg.Section("service").Key("REGISTER_EMAIL_CONFIRM").MustBool() {
		return
	} else if MailService == nil {
		log.Warn("Register Mail Service: Mail Service is not enabled")
		return
	}
	Service.RegisterEmailConfirm = true
	log.Info("Register Mail Service Enabled")
}

func newNotifyMailService() {
	if !Cfg.Section("service").Key("ENABLE_NOTIFY_MAIL").MustBool() {
		return
	} else if MailService == nil {
		log.Warn("Notify Mail Service: Mail Service is not enabled")
		return
	}
	Service.EnableNotifyMail = true
	log.Info("Notify Mail Service Enabled")
}

func tryResolveAddr(addr string) []net.IP {
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		addr = addr[1 : len(addr)-1]
	}
	ip := net.ParseIP(addr)
	if ip != nil {
		ips := make([]net.IP, 1)
		ips[0] = ip
		return ips
	}
	ips, err := net.LookupIP(addr)
	if err != nil {
		log.Warn("could not look up mailer.SMTP_ADDR: %v", err)
		return make([]net.IP, 0)
	}
	return ips
}
