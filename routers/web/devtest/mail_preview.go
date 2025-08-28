// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/mailer"

	"gopkg.in/yaml.v3"
)

func MailPreviewRender(ctx *context.Context) {
	tmplName := ctx.PathParam("*")
	mockDataContent, err := templates.AssetFS().ReadFile("mail/" + tmplName + ".devtest.yml")
	mockData := map[string]any{}
	if err == nil {
		err = yaml.Unmarshal(mockDataContent, &mockData)
		if err != nil {
			http.Error(ctx.Resp, "Failed to parse mock data: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	mockData["locale"] = ctx.Locale
	err = mailer.LoadedTemplates().BodyTemplates.ExecuteTemplate(ctx.Resp, tmplName, mockData)
	if err != nil {
		_, _ = ctx.Resp.Write([]byte(err.Error()))
	}
}

func prepareMailPreviewRender(ctx *context.Context, tmplName string) {
	tmplSubject := mailer.LoadedTemplates().SubjectTemplates.Lookup(tmplName)
	if tmplSubject == nil {
		ctx.Data["RenderMailSubject"] = "default subject"
	} else {
		var buf strings.Builder
		err := tmplSubject.Execute(&buf, nil)
		if err != nil {
			ctx.Data["RenderMailSubject"] = err.Error()
		} else {
			ctx.Data["RenderMailSubject"] = buf.String()
		}
	}
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
