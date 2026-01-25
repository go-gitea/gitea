// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"html/template"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type MailRender struct {
	TemplateNames []string
	BodyTemplates struct {
		HasTemplate     func(name string) bool
		ExecuteTemplate func(w io.Writer, name string, data any) error
	}

	// FIXME: MAIL-TEMPLATE-SUBJECT: only "issue" related messages support using subject from templates
	// It is an incomplete implementation from "Use templates for issue e-mail subject and body" https://github.com/go-gitea/gitea/pull/8329
	SubjectTemplates *texttmpl.Template

	tmplRenderer *tmplRender

	mockedBodyTemplates map[string]*template.Template
}

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

var mailSubjectSplit = regexp.MustCompile(`(?m)^-{3,}\s*$`)

func newMailRenderer() (*MailRender, error) {
	subjectTemplates := texttmpl.New("")
	subjectTemplates.Funcs(mailSubjectTextFuncMap())

	renderer := &MailRender{
		SubjectTemplates: subjectTemplates,
	}

	assetFS := AssetFS()

	renderer.tmplRenderer = &tmplRender{
		collectTemplateNames: func() ([]string, error) {
			names, err := assetFS.ListAllFiles(".", true)
			if err != nil {
				return nil, err
			}
			names = slices.DeleteFunc(names, func(file string) bool {
				return !strings.HasPrefix(file, "mail/") || !strings.HasSuffix(file, ".tmpl")
			})
			for i, name := range names {
				names[i] = strings.TrimSuffix(strings.TrimPrefix(name, "mail/"), ".tmpl")
			}
			renderer.TemplateNames = names
			return names, nil
		},
		readTemplateContent: func(name string) ([]byte, error) {
			content, err := assetFS.ReadFile("mail/" + name + ".tmpl")
			if err != nil {
				return nil, err
			}
			var subjectContent []byte
			bodyContent := content
			loc := mailSubjectSplit.FindIndex(content)
			if loc != nil {
				subjectContent, bodyContent = content[0:loc[0]], content[loc[1]:]
			}
			_, err = renderer.SubjectTemplates.New(name).Parse(string(subjectContent))
			if err != nil {
				return nil, err
			}
			return bodyContent, nil
		},
	}

	renderer.BodyTemplates.HasTemplate = func(name string) bool {
		if renderer.mockedBodyTemplates[name] != nil {
			return true
		}
		return renderer.tmplRenderer.Templates().HasTemplate(name)
	}

	staticFuncMap := NewFuncMap()
	renderer.BodyTemplates.ExecuteTemplate = func(w io.Writer, name string, data any) error {
		if t, ok := renderer.mockedBodyTemplates[name]; ok {
			return t.Execute(w, data)
		}
		t, err := renderer.tmplRenderer.Templates().Executor(name, staticFuncMap)
		if err != nil {
			return err
		}
		return t.Execute(w, data)
	}

	err := renderer.tmplRenderer.recompileTemplates(staticFuncMap)
	if err != nil {
		return nil, err
	}
	return renderer, nil
}

func (r *MailRender) MockTemplate(name, subject, body string) func() {
	if r.mockedBodyTemplates == nil {
		r.mockedBodyTemplates = make(map[string]*template.Template)
	}
	oldSubject := r.SubjectTemplates
	r.SubjectTemplates, _ = r.SubjectTemplates.Clone()
	texttmpl.Must(r.SubjectTemplates.New(name).Parse(subject))

	oldBody, hasOldBody := r.mockedBodyTemplates[name]
	mockFuncMap := NewFuncMap()
	r.mockedBodyTemplates[name] = template.Must(template.New(name).Funcs(mockFuncMap).Parse(body))
	return func() {
		r.SubjectTemplates = oldSubject
		if hasOldBody {
			r.mockedBodyTemplates[name] = oldBody
		} else {
			delete(r.mockedBodyTemplates, name)
		}
	}
}

var (
	globalMailRenderer   *MailRender
	globalMailRendererMu sync.RWMutex
)

func MailRendererReload() error {
	globalMailRendererMu.Lock()
	defer globalMailRendererMu.Unlock()
	r, err := newMailRenderer()
	if err != nil {
		return err
	}
	globalMailRenderer = r
	return nil
}

func MailRenderer() *MailRender {
	globalMailRendererMu.RLock()
	r := globalMailRenderer
	globalMailRendererMu.RUnlock()
	if r != nil {
		return r
	}

	globalMailRendererMu.Lock()
	defer globalMailRendererMu.Unlock()
	if globalMailRenderer != nil {
		return globalMailRenderer
	}

	var err error
	globalMailRenderer, err = newMailRenderer()
	if err != nil {
		log.Fatal("Failed to initialize mail renderer: %v", err)
	}

	if !setting.IsProd {
		go AssetFS().WatchLocalChanges(graceful.GetManager().ShutdownContext(), func() {
			globalMailRendererMu.Lock()
			defer globalMailRendererMu.Unlock()
			r, err := newMailRenderer()
			if err != nil {
				log.Error("Mail template error: %v", err)
				return
			}
			globalMailRenderer = r
		})
	}
	return globalMailRenderer
}
