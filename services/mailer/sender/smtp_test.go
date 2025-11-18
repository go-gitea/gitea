package sender

import (
	"context"
	"io"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	gomail "github.com/wneessen/go-mail"
	"github.com/wneessen/go-mail/smtp"
)

type writerToFunc func(io.Writer) (int64, error)

func (f writerToFunc) WriteTo(w io.Writer) (int64, error) {
	return f(w)
}

type fakeGomailClient struct {
	dialAndSendCalled bool
	dialAndSendErr    error
	closeCalled       bool
	probeCalled       bool
	setAuthCalled     bool
	authType          gomail.SMTPAuthType
	setCustomCalled   bool
	probeErr          error
}

func (f *fakeGomailClient) Close() error {
	f.closeCalled = true
	return nil
}

func (f *fakeGomailClient) DialAndSend(_ ...*gomail.Msg) error {
	f.dialAndSendCalled = true
	return f.dialAndSendErr
}

func (f *fakeGomailClient) DialToSMTPClientWithContext(context.Context) (*smtp.Client, error) {
	f.probeCalled = true
	return &smtp.Client{}, f.probeErr
}

func (f *fakeGomailClient) CloseWithSMTPClient(*smtp.Client) error {
	return nil
}

func (f *fakeGomailClient) SetSMTPAuth(auth gomail.SMTPAuthType) {
	f.setAuthCalled = true
	f.authType = auth
}

func (f *fakeGomailClient) SetSMTPAuthCustom(smtp.Auth) {
	f.setCustomCalled = true
}

func TestSMTPSenderRejectsNonGomailMessage(t *testing.T) {
	oldConf := setting.MailService
	t.Cleanup(func() {
		setting.MailService = oldConf
	})
	setting.MailService = &setting.Mailer{
		Protocol: "smtp",
		SMTPAddr: "localhost",
		SMTPPort: "25",
	}

	err := new(SMTPSender).Send("", nil, writerToFunc(func(io.Writer) (int64, error) {
		return 0, nil
	}))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected message type")
}

func TestSMTPSenderAuthUnsupported(t *testing.T) {
	fakeClient := &fakeGomailClient{}
	overrideClient := func(host string, opts ...gomail.Option) (gomailClient, error) {
		return fakeClient, nil
	}
	oldClientFactory := newGomailClient
	oldProbe := probeSMTPServerFunc
	t.Cleanup(func() {
		newGomailClient = oldClientFactory
		probeSMTPServerFunc = oldProbe
	})
	newGomailClient = overrideClient
	probeSMTPServerFunc = func(gomailClient) (bool, string, bool, error) {
		return false, "", false, nil
	}

	oldConf := setting.MailService
	t.Cleanup(func() {
		setting.MailService = oldConf
	})
	setting.MailService = &setting.Mailer{
		Protocol: "smtp",
		SMTPAddr: "smtp.example.com",
		SMTPPort: "25",
		User:     "user",
		Passwd:   "pass",
	}

	msg := gomail.NewMsg()
	msg.SetBodyString("text/plain", "body")

	err := new(SMTPSender).Send("", nil, msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support AUTH")
	assert.False(t, fakeClient.dialAndSendCalled)
	assert.True(t, fakeClient.closeCalled)
}

func TestSMTPSenderAuthAutoDiscover(t *testing.T) {
	fakeClient := &fakeGomailClient{}
	overrideClient := func(host string, opts ...gomail.Option) (gomailClient, error) {
		return fakeClient, nil
	}
	oldClientFactory := newGomailClient
	oldProbe := probeSMTPServerFunc
	t.Cleanup(func() {
		newGomailClient = oldClientFactory
		probeSMTPServerFunc = oldProbe
	})
	newGomailClient = overrideClient
	probeSMTPServerFunc = func(gomailClient) (bool, string, bool, error) {
		return true, "SCRAM-SHA-256 XOAUTH2", true, nil
	}

	oldConf := setting.MailService
	t.Cleanup(func() {
		setting.MailService = oldConf
	})
	setting.MailService = &setting.Mailer{
		Protocol: "smtp+starttls",
		SMTPAddr: "smtp.example.com",
		SMTPPort: "587",
		User:     "user",
		Passwd:   "pass",
	}

	msg := gomail.NewMsg()
	msg.SetBodyString("text/plain", "body")

	err := new(SMTPSender).Send("", nil, msg)

	assert.NoError(t, err)
	assert.True(t, fakeClient.setAuthCalled)
	assert.Equal(t, gomail.SMTPAuthAutoDiscover, fakeClient.authType)
	assert.False(t, fakeClient.setCustomCalled)
	assert.True(t, fakeClient.dialAndSendCalled)
	assert.True(t, fakeClient.closeCalled)
}
