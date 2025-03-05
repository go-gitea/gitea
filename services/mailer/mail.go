// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"mime"
	"regexp"
	"strings"
	texttmpl "text/template"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/typesniffer"
	sender_service "code.gitea.io/gitea/services/mailer/sender"

	"golang.org/x/net/html"
)

const mailMaxSubjectRunes = 256 // There's no actual limit for subject in RFC 5322

var (
	bodyTemplates       *template.Template
	subjectTemplates    *texttmpl.Template
	subjectRemoveSpaces = regexp.MustCompile(`[\s]+`)
)

// SendTestMail sends a test mail
func SendTestMail(email string) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}
	return sender_service.Send(sender, sender_service.NewMessage(email, "Gitea Test Email!", "Gitea Test Email!"))
}

func sanitizeSubject(subject string) string {
	runes := []rune(strings.TrimSpace(subjectRemoveSpaces.ReplaceAllLiteralString(subject, " ")))
	if len(runes) > mailMaxSubjectRunes {
		runes = runes[:mailMaxSubjectRunes]
	}
	// Encode non-ASCII characters
	return mime.QEncoding.Encode("utf-8", string(runes))
}

type mailAttachmentBase64Embedder struct {
	doer         *user_model.User
	repo         *repo_model.Repository
	maxSize      int64
	estimateSize int64
}

func newMailAttachmentBase64Embedder(doer *user_model.User, repo *repo_model.Repository, maxSize int64) *mailAttachmentBase64Embedder {
	return &mailAttachmentBase64Embedder{doer: doer, repo: repo, maxSize: maxSize}
}

func (b64embedder *mailAttachmentBase64Embedder) Base64InlineImages(ctx context.Context, body template.HTML) (template.HTML, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("html.Parse failed: %w", err)
	}

	b64embedder.estimateSize = int64(len(string(body)))

	var processNode func(*html.Node)
	processNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "img" {
				for i, attr := range n.Attr {
					if attr.Key == "src" {
						attachmentSrc := attr.Val
						dataURI, err := b64embedder.AttachmentSrcToBase64DataURI(ctx, attachmentSrc)
						if err != nil {
							// Not an error, just skip. This is probably an image from outside the gitea instance.
							log.Trace("Unable to embed attachment %q to mail body: %v", attachmentSrc, err)
						} else {
							n.Attr[i].Val = dataURI
						}
						break
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			processNode(c)
		}
	}

	processNode(doc)

	var buf bytes.Buffer
	err = html.Render(&buf, doc)
	if err != nil {
		return "", fmt.Errorf("html.Render failed: %w", err)
	}
	return template.HTML(buf.String()), nil
}

func (b64embedder *mailAttachmentBase64Embedder) AttachmentSrcToBase64DataURI(ctx context.Context, attachmentSrc string) (string, error) {
	parsedSrc := httplib.ParseGiteaSiteURL(ctx, attachmentSrc)
	var attachmentUUID string
	if parsedSrc != nil {
		var ok bool
		attachmentUUID, ok = strings.CutPrefix(parsedSrc.RoutePath, "/attachments/")
		if !ok {
			attachmentUUID, ok = strings.CutPrefix(parsedSrc.RepoSubPath, "/attachments/")
		}
		if !ok {
			return "", fmt.Errorf("not an attachment")
		}
	}
	attachment, err := repo_model.GetAttachmentByUUID(ctx, attachmentUUID)
	if err != nil {
		return "", err
	}

	if attachment.RepoID != b64embedder.repo.ID {
		return "", fmt.Errorf("attachment does not belong to the repository")
	}
	if attachment.Size+b64embedder.estimateSize > b64embedder.maxSize {
		return "", fmt.Errorf("total embedded images exceed max limit")
	}

	fr, err := storage.Attachments.Open(attachment.RelativePath())
	if err != nil {
		return "", err
	}
	defer fr.Close()

	lr := &io.LimitedReader{R: fr, N: b64embedder.maxSize + 1}
	content, err := io.ReadAll(lr)
	if err != nil {
		return "", fmt.Errorf("LimitedReader ReadAll: %w", err)
	}

	mimeType := typesniffer.DetectContentType(content)
	if !mimeType.IsImage() {
		return "", fmt.Errorf("not an image")
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType.GetMimeType(), encoded)
	b64embedder.estimateSize += int64(len(dataURI))
	return dataURI, nil
}

func fromDisplayName(u *user_model.User) string {
	if setting.MailService.FromDisplayNameFormatTemplate != nil {
		var ctx bytes.Buffer
		err := setting.MailService.FromDisplayNameFormatTemplate.Execute(&ctx, map[string]any{
			"DisplayName": u.DisplayName(),
			"AppName":     setting.AppName,
			"Domain":      setting.Domain,
		})
		if err == nil {
			return mime.QEncoding.Encode("utf-8", ctx.String())
		}
		log.Error("fromDisplayName: %w", err)
	}
	return u.GetCompleteName()
}
