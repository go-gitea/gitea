// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"sync/atomic"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type MailTemplates struct {
	TemplateNames    []string
	BodyTemplates    *template.Template
	SubjectTemplates *texttmpl.Template
}

var mailSubjectSplit = regexp.MustCompile(`(?m)^-{3,}\s*$`)

// mailSubjectTextFuncMap returns functions for injecting to text templates, it's only used for mail subject
func mailSubjectTextFuncMap() texttmpl.FuncMap {
	return texttmpl.FuncMap{
		"dict": dict,
		"Eval": evalTokens,

		"EllipsisString": util.EllipsisDisplayString,
		"AppName": func() string {
			return setting.AppName
		},
		"AppDomain": func() string { // documented in mail-templates.md
			return setting.Domain
		},
	}
}

func buildSubjectBodyTemplate(stpl *texttmpl.Template, btpl *template.Template, name string, content []byte) error {
	// Split template into subject and body
	var subjectContent []byte
	bodyContent := content
	loc := mailSubjectSplit.FindIndex(content)
	if loc != nil {
		subjectContent = content[0:loc[0]]
		bodyContent = content[loc[1]:]
	}
	if _, err := stpl.New(name).Parse(string(subjectContent)); err != nil {
		return fmt.Errorf("failed to parse template [%s/subject]: %w", name, err)
	}
	if _, err := btpl.New(name).Parse(string(bodyContent)); err != nil {
		return fmt.Errorf("failed to parse template [%s/body]: %w", name, err)
	}
	return nil
}

// LoadMailTemplates provides the templates required for sending notification mails.
func LoadMailTemplates(ctx context.Context, loadedTemplates *atomic.Pointer[MailTemplates]) {
	assetFS := AssetFS()
	refreshTemplates := func(firstRun bool) {
		var templateNames []string
		subjectTemplates := texttmpl.New("")
		bodyTemplates := template.New("")

		subjectTemplates.Funcs(mailSubjectTextFuncMap())
		bodyTemplates.Funcs(NewFuncMap())

		if !firstRun {
			log.Trace("Reloading mail templates")
		}
		assetPaths, err := ListMailTemplateAssetNames(assetFS)
		if err != nil {
			log.Error("Failed to list mail templates: %v", err)
			return
		}

		for _, assetPath := range assetPaths {
			content, layerName, err := assetFS.ReadLayeredFile(assetPath)
			if err != nil {
				log.Warn("Failed to read mail template %s by %s: %v", assetPath, layerName, err)
				continue
			}
			tmplName := strings.TrimPrefix(strings.TrimSuffix(assetPath, ".tmpl"), "mail/")
			if firstRun {
				log.Trace("Adding mail template %s: %s by %s", tmplName, assetPath, layerName)
			}
			templateNames = append(templateNames, tmplName)
			if err = buildSubjectBodyTemplate(subjectTemplates, bodyTemplates, tmplName, content); err != nil {
				if firstRun {
					log.Fatal("Failed to parse mail template, err: %v", err)
				}
				log.Error("Failed to parse mail template, err: %v", err)
			}
		}
		loaded := &MailTemplates{
			TemplateNames:    templateNames,
			BodyTemplates:    bodyTemplates,
			SubjectTemplates: subjectTemplates,
		}
		loadedTemplates.Store(loaded)
	}

	refreshTemplates(true)

	if !setting.IsProd {
		// Now subjectTemplates and bodyTemplates are both synchronized
		// thus it is safe to call refresh from a different goroutine
		go assetFS.WatchLocalChanges(ctx, func() {
			refreshTemplates(false)
		})
	}
}
