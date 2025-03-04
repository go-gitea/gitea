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
	"net/http"
	"regexp"
	"strings"
	texttmpl "text/template"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
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
	doer    *user_model.User
	repo    *repo_model.Repository
	maxSize int64
}

func newMailAttachmentBase64Embedder(doer *user_model.User, repo *repo_model.Repository, maxSize int64) *mailAttachmentBase64Embedder {
	return &mailAttachmentBase64Embedder{doer: doer, repo: repo, maxSize: maxSize}
}

func (b64embedder *mailAttachmentBase64Embedder) Base64InlineImages(ctx context.Context, body string) (string, error) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	var totalEmbeddedImagesSize int64

	var processNode func(*html.Node)
	processNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "img" {
				for i, attr := range n.Attr {
					if attr.Key == "src" {
						attachmentPath := attr.Val
						dataURI, err := b64embedder.AttachmentSrcToBase64DataURI(ctx, attachmentPath, &totalEmbeddedImagesSize)
						if err != nil {
							log.Trace("attachmentSrcToDataURI not possible: %v", err) // Not an error, just skip. This is probably an image from outside the gitea instance.
							continue
						}
						n.Attr[i].Val = dataURI
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
		log.Error("Failed to render modified HTML: %v", err)
		return "", err
	}
	return buf.String(), nil
}

func (b64embedder *mailAttachmentBase64Embedder) AttachmentSrcToBase64DataURI(ctx context.Context, attachmentPath string, totalEmbeddedImagesSize *int64) (string, error) {
	if !strings.HasPrefix(attachmentPath, setting.AppURL) { // external image
		return "", fmt.Errorf("external image")
	}
	parts := strings.Split(attachmentPath, "/attachments/")
	if len(parts) <= 1 {
		return "", fmt.Errorf("invalid attachment path: %s", attachmentPath)
	}

	attachmentUUID := parts[len(parts)-1]
	attachment, err := repo_model.GetAttachmentByUUID(ctx, attachmentUUID)
	if err != nil {
		return "", err
	}

	if attachment.RepoID != b64embedder.repo.ID {
		return "", fmt.Errorf("attachment does not belong to the repository")
	}

	fr, err := storage.Attachments.Open(attachment.RelativePath())
	if err != nil {
		return "", err
	}
	defer fr.Close()

	lr := &io.LimitedReader{R: fr, N: b64embedder.maxSize + 1}
	content, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	if int64(len(content)) > b64embedder.maxSize {
		return "", fmt.Errorf("file size exceeds the embedded image max limit \\(%d bytes\\)", b64embedder.maxSize)
	}

	if *totalEmbeddedImagesSize+int64(len(content)) > setting.MailService.Base64EmbedImagesMaxSizePerEmail {
		return "", fmt.Errorf("total embedded images exceed max limit: %d > %d", *totalEmbeddedImagesSize+int64(len(content)), setting.MailService.Base64EmbedImagesMaxSizePerEmail)
	}
	*totalEmbeddedImagesSize += int64(len(content))

	mimeType := http.DetectContentType(content)

	if !strings.HasPrefix(mimeType, "image/") {
		return "", fmt.Errorf("not an image")
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

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
