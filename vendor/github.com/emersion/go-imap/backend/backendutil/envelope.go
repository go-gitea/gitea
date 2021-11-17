package backendutil

import (
	"net/mail"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/textproto"
)

func headerAddressList(value string) ([]*imap.Address, error) {
	addrs, err := mail.ParseAddressList(value)
	if err != nil {
		return []*imap.Address{}, err
	}

	list := make([]*imap.Address, len(addrs))
	for i, a := range addrs {
		parts := strings.SplitN(a.Address, "@", 2)
		mailbox := parts[0]
		var hostname string
		if len(parts) == 2 {
			hostname = parts[1]
		}

		list[i] = &imap.Address{
			PersonalName: a.Name,
			MailboxName:  mailbox,
			HostName:     hostname,
		}
	}

	return list, err
}

// FetchEnvelope returns a message's envelope from its header.
func FetchEnvelope(h textproto.Header) (*imap.Envelope, error) {
	env := new(imap.Envelope)

	env.Date, _ = mail.ParseDate(h.Get("Date"))
	env.Subject = h.Get("Subject")
	env.From, _ = headerAddressList(h.Get("From"))
	env.Sender, _ = headerAddressList(h.Get("Sender"))
	if len(env.Sender) == 0 {
		env.Sender = env.From
	}
	env.ReplyTo, _ = headerAddressList(h.Get("Reply-To"))
	if len(env.ReplyTo) == 0 {
		env.ReplyTo = env.From
	}
	env.To, _ = headerAddressList(h.Get("To"))
	env.Cc, _ = headerAddressList(h.Get("Cc"))
	env.Bcc, _ = headerAddressList(h.Get("Bcc"))
	env.InReplyTo = h.Get("In-Reply-To")
	env.MessageId = h.Get("Message-Id")

	return env, nil
}
