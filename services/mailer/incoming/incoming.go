// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package incoming

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	net_mail "net/mail"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/mailer/token"

	"github.com/dimiro1/reply"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/jaytaylor/html2text"
)

var (
	addressTokenRegex   *regexp.Regexp
	referenceTokenRegex *regexp.Regexp
)

func Init(ctx context.Context) error {
	if !setting.IncomingEmail.Enabled {
		return nil
	}

	addressTokenRegex = regexp.MustCompile(
		fmt.Sprintf(
			`\A%s\z`,
			strings.Replace(regexp.QuoteMeta(setting.IncomingEmail.ReplyToAddress), regexp.QuoteMeta(setting.IncomingEmail.TokenPlaceholder), "(.+)", 1),
		),
	)
	referenceTokenRegex = regexp.MustCompile(fmt.Sprintf(`\Areply-(.+)@%s\z`, regexp.QuoteMeta(setting.Domain)))

	go func() {
		ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Incoming Email", process.SystemProcessType, true)
		defer finished()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := processIncomingEmails(ctx); err != nil {
					log.Error("Error while processing incoming emails: %v", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.NewTimer(10 * time.Second).C:
				}
			}
		}
	}()

	return nil
}

// processIncomingEmails is the "main" method with the wait/process loop
func processIncomingEmails(ctx context.Context) error {
	server := fmt.Sprintf("%s:%d", setting.IncomingEmail.Host, setting.IncomingEmail.Port)

	var c *client.Client
	var err error
	if setting.IncomingEmail.UseTLS {
		c, err = client.DialTLS(server, &tls.Config{InsecureSkipVerify: setting.IncomingEmail.SkipTLSVerify})
	} else {
		c, err = client.Dial(server)
	}
	if err != nil {
		return fmt.Errorf("connected failed: %w", err)
	}

	if err := c.Login(setting.IncomingEmail.Username, setting.IncomingEmail.Password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	defer func() {
		if err := c.Logout(); err != nil {
			log.Error("Logout failed: %v", err)
		}
	}()

	if _, err := c.Select(setting.IncomingEmail.Mailbox, false); err != nil {
		return fmt.Errorf("selecting box '%s' failed: %w", setting.IncomingEmail.Mailbox, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := processMessages(ctx, c); err != nil {
				return fmt.Errorf("do it failed: %w", err)
			}
			if err := waitForUpdates(ctx, c); err != nil {
				return fmt.Errorf("wait for updates failed: %w", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.NewTimer(time.Second).C:
			}
		}
	}
}

// waitForUpdates uses IMAP IDLE to wait for new emails
func waitForUpdates(ctx context.Context, c *client.Client) error {
	updates := make(chan client.Update, 1)

	c.Updates = updates
	defer func() {
		c.Updates = nil
	}()

	errs := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		errs <- c.Idle(stop, nil)
	}()

	stopped := false
	for {
		select {
		case update := <-updates:
			switch update.(type) {
			case *client.MailboxUpdate:
				if !stopped {
					close(stop)
					stopped = true
				}
			default:
			}
		case err := <-errs:
			if err != nil {
				return fmt.Errorf("imap idle failed: %w", err)
			}
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

// processMessages searches unread mails and processes them.
func processMessages(ctx context.Context, c *client.Client) error {
	mbox, err := c.Select(setting.IncomingEmail.Mailbox, false)
	if err != nil {
		return fmt.Errorf("selecting box '%s' failed: %w", setting.IncomingEmail.Mailbox, err)
	}

	if mbox.Messages == 0 {
		return nil
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	criteria.Smaller = setting.IncomingEmail.MaximumMessageSize
	ids, err := c.Search(criteria)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(ids) == 0 {
		return nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	messages := make(chan *imap.Message, 10)

	section := &imap.BodySectionName{}

	errs := make(chan error, 1)
	go func() {
		errs <- c.Fetch(
			seqset,
			[]imap.FetchItem{section.FetchItem()},
			messages,
		)
	}()

	handledSet := new(imap.SeqSet)
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case msg, ok := <-messages:
			if !ok {
				if setting.IncomingEmail.DeleteHandledMessage && !handledSet.Empty() {
					if err := c.Store(
						handledSet,
						imap.FormatFlagsOp(imap.AddFlags, true),
						[]interface{}{imap.DeletedFlag},
						nil,
					); err != nil {
						return fmt.Errorf("store failed: %w", err)
					}

					if err := c.Expunge(nil); err != nil {
						return fmt.Errorf("expunge failed: %w", err)
					}
				}
				return nil
			}

			err := func() error {
				r := msg.GetBody(section)
				if r == nil {
					return fmt.Errorf("get body failed: %w", err)
				}

				mr, err := mail.CreateReader(r)
				if err != nil {
					return fmt.Errorf("create reader failed: %w", err)
				}
				defer mr.Close()

				if isAutomaticReply(mr.Header) {
					log.Debug("Skipping automatic reply")
					return nil
				}

				t := searchTokenInHeaders(mr.Header)
				if t == "" {
					log.Debug("Token not found")
					return nil
				}

				handlerType, user, payload, err := token.ExtractToken(ctx, t)
				if err != nil {
					if _, ok := err.(*token.ErrToken); ok {
						log.Info("Invalid email token: %v", err)
						return nil
					}
					return err
				}

				handler, ok := handlers[handlerType]
				if !ok {
					return fmt.Errorf("unexpected handler type: %v", handlerType)
				}

				content, err := getContentFromMailReader(mr)
				if err != nil {
					return fmt.Errorf("getContentFromMailReader failed: %w", err)
				}

				if err := handler.Handle(ctx, content, user, payload); err != nil {
					return fmt.Errorf("Handle failed: %w", err)
				}

				handledSet.AddNum(msg.SeqNum)

				return nil
			}()
			if err != nil {
				log.Error("Error while processing message[]: %v", err)
			}
		}
	}

	if err := <-errs; err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	return nil
}

// isAutomaticReply tests if the headers indicate an automatic reply
func isAutomaticReply(h mail.Header) bool {
	autoSubmitted := h.Get("Auto-Submitted")
	if autoSubmitted != "" && autoSubmitted != "no" {
		return true
	}
	autoReply := h.Get("X-Autoreply")
	if autoReply == "yes" {
		return true
	}
	autoRespond := h.Get("X-Autorespond")
	return autoRespond != ""
}

// searchTokenInHeaders looks for the token in To, Delivered-To and References
func searchTokenInHeaders(h mail.Header) string {
	if addressTokenRegex != nil {
		to, _ := h.AddressList("To")
		deliveredTo, _ := h.AddressList("Delivered-To")
		for _, list := range [][]*net_mail.Address{
			to,
			deliveredTo,
		} {
			for _, address := range list {
				match := addressTokenRegex.FindStringSubmatch(address.Address)
				if len(match) != 2 {
					continue
				}

				return match[1]
			}
		}
	}

	references := h.Get("References")
	for {
		begin := strings.IndexByte(references, '<')
		if begin == -1 {
			break
		}
		begin++

		end := strings.IndexByte(references, '>')
		if end == -1 || begin > end {
			break
		}

		match := referenceTokenRegex.FindStringSubmatch(references[begin:end])
		if len(match) == 2 {
			return match[1]
		}

		references = references[end+1:]
	}

	return ""
}

type MailContent struct {
	Content     string
	Attachments []*Attachment
}

type Attachment struct {
	Name    string
	Content bytes.Buffer
}

// getContentFromMailReader reads the plain content and the attachments from the mail.
// If there is only HTML content, it gets converted to plain text.
// A potential reply/signature gets stripped from the content.
func getContentFromMailReader(mr *mail.Reader) (*MailContent, error) {
	contentText := ""
	contentHTML := ""
	attachments := make([]*Attachment, 0, 1)

	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("next part failed: %w", err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, err := h.ContentType()
			if err != nil {
				return nil, fmt.Errorf("ContentType failed: %w", err)
			}

			if contentType == "text/plain" {
				if contentText != "" {
					continue
				}
			} else if contentType == "text/html" {
				if contentHTML != "" {
					continue
				}
			} else {
				continue
			}

			data, err := io.ReadAll(p.Body)
			if err != nil {
				return nil, fmt.Errorf("read body failed: %w", err)
			}

			switch contentType {
			case "text/plain":
				contentText = string(data)
			case "text/html":
				contentHTML = string(data)
			}
		case *mail.AttachmentHeader:
			attachment := &Attachment{}
			attachment.Name, _ = h.Filename()
			_, err := io.Copy(&attachment.Content, p.Body)
			if err != nil {
				return nil, fmt.Errorf("read attachment failed: %w", err)
			}

			attachments = append(attachments, attachment)
		}
	}

	if contentText == "" && contentHTML != "" {
		var err error
		contentText, err = html2text.FromString(contentHTML)
		if err != nil {
			return nil, err
		}
	}

	return &MailContent{
		Content:     reply.FromText(contentText),
		Attachments: attachments,
	}, nil
}
