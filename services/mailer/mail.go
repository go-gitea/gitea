// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	texttmpl "text/template"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
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

func Base64InlineImages(body string, ctx *mailCommentContext) (string, error) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		log.Error("Failed to parse HTML body: %v", err)
		return "", err
	}

	var totalEmbeddedImagesSize int64

	var processNode func(*html.Node)
	processNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "img" {
				for i, attr := range n.Attr {
					if attr.Key == "src" {
						attachmentPath := attr.Val
						dataURI, err := AttachmentSrcToBase64DataURI(attachmentPath, ctx, &totalEmbeddedImagesSize)
						if err != nil {
							log.Trace("attachmentSrcToDataURI not possible: %v", err) // Not an error, just skip. This is probably an image from outside the gitea instance.
							continue
						}
						log.Trace("Old value of src attribute: %s, new value (first 100 characters): %s", attr.Val, dataURI[:100])
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

func AttachmentSrcToBase64DataURI(attachmentPath string, ctx *mailCommentContext, totalEmbeddedImagesSize *int64) (string, error) {
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

	// "Doer" is theoretically not the correct permission check (as Doer created the action on which to send), but as this is batch processed the receipants can't be accessed.
	// Therefore we check the Doer, with which we counter leaking information as a Doer brute force attack on attachments would be possible.
	perm, err := access_model.GetUserRepoPermission(ctx, ctx.Issue.Repo, ctx.Doer)
	if err != nil {
		return "", err
	}
	if !perm.CanRead(unit.TypeIssues) {
		return "", fmt.Errorf("no permission")
	}

	fr, err := storage.Attachments.Open(attachment.RelativePath())
	if err != nil {
		return "", err
	}
	defer fr.Close()

	maxSize := setting.MailService.Base64EmbedImagesMaxSizePerEmail // at maximum read the whole available combined email size, to prevent maliciously large file reads

	lr := &io.LimitedReader{R: fr, N: maxSize + 1}
	content, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	if len(content) > int(maxSize) {
		return "", fmt.Errorf("file size exceeds the embedded image max limit \\(%d bytes\\)", maxSize)
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
