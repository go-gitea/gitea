// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/mailer"

	"go.yaml.in/yaml/v4"
)

const mailDarkSchemeQuery = "@media (prefers-color-scheme: dark)"

func mailPreviewMockData(tmplName string) (map[string]any, error) {
	mockData := map[string]any{}
	mockDataContent, err := templates.AssetFS().ReadFile("mail/" + tmplName + ".devtest.yml")
	if err != nil {
		return mockData, nil
	}
	return mockData, yaml.Unmarshal(mockDataContent, &mockData)
}

func MailPreviewRender(ctx *context.Context) {
	tmplName := ctx.PathParam("*")
	mockData, err := mailPreviewMockData(tmplName)
	if err != nil {
		http.Error(ctx.Resp, "Failed to parse mock data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	mockData["locale"] = ctx.Locale
	var mailBody bytes.Buffer
	err = mailer.LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, tmplName, mockData)
	if err != nil {
		_, _ = ctx.Resp.Write([]byte(err.Error()))
		return
	}
	body := mailBody.String()
	// a page can force "color-scheme" on an embedded document but never "prefers-color-scheme"
	if scheme := ctx.FormString("scheme"); scheme == "light" || scheme == "dark" {
		body = strings.ReplaceAll(body, mailDarkSchemeQuery, util.Iif(scheme == "dark", "@media all", "@media not all"))
		body += fmt.Sprintf(`<style>:root {color-scheme: %s}</style>`, scheme)
	}
	_, _ = ctx.Resp.Write([]byte(body))
}

func prepareMailPreviewRender(ctx *context.Context, tmplName string) {
	subject := "(default subject)"
	if mockData, err := mailPreviewMockData(tmplName); err == nil {
		if mockSubject, ok := mockData["Subject"].(string); ok {
			subject = util.IfZero(mockSubject, subject)
		}
	}
	tmplSubject := mailer.LoadedTemplates().SubjectTemplates.Lookup(tmplName)
	// FIXME: MAIL-TEMPLATE-SUBJECT: only "issue" related messages support using subject from templates
	if tmplSubject != nil {
		var buf strings.Builder
		err := tmplSubject.Execute(&buf, nil)
		if err != nil {
			subject = "ERROR: " + err.Error()
		} else {
			subject = util.IfZero(buf.String(), subject)
		}
	}
	ctx.Data["RenderMailSubject"] = subject
	ctx.Data["RenderMailTemplateName"] = tmplName
}

func MailPreview(ctx *context.Context) {
	ctx.Data["MailTemplateNames"] = mailer.LoadedTemplates().TemplateNames
	tmplName := ctx.FormString("tmpl")
	if tmplName != "" {
		prepareMailPreviewRender(ctx, tmplName)
	}
	ctx.HTML(http.StatusOK, "devtest/mail-preview")
}
