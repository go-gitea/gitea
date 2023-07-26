// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package scopedtmpl

import (
	"fmt"
	"html/template"
	"io"
	"reflect"
	"sync"
	texttemplate "text/template"
	"text/template/parse"
	"unsafe"
)

type TemplateExecutor interface {
	Execute(wr io.Writer, data any) error
}

type ScopedTemplate struct {
	all        *template.Template
	parseFuncs template.FuncMap // this func map is only used for parsing templates
	frozen     bool

	scopedMu           sync.RWMutex
	scopedTemplateSets map[string]*scopedTemplateSet
}

func NewScopedTemplate() *ScopedTemplate {
	return &ScopedTemplate{
		all:                template.New(""),
		parseFuncs:         template.FuncMap{},
		scopedTemplateSets: map[string]*scopedTemplateSet{},
	}
}

func (t *ScopedTemplate) Funcs(funcMap template.FuncMap) {
	if t.frozen {
		panic("cannot add new functions to frozen template set")
	}
	t.all.Funcs(funcMap)
	for k, v := range funcMap {
		t.parseFuncs[k] = v
	}
}

func (t *ScopedTemplate) New(name string) *template.Template {
	if t.frozen {
		panic("cannot add new template to frozen template set")
	}
	return t.all.New(name)
}

func (t *ScopedTemplate) Freeze() {
	t.frozen = true
	// reset the exec func map, then `escapeTemplate` is safe to call `Execute` to do escaping
	m := template.FuncMap{}
	for k := range t.parseFuncs {
		m[k] = func(v ...any) any { return nil }
	}
	t.all.Funcs(m)
}

func (t *ScopedTemplate) Executor(name string, funcMap template.FuncMap) (TemplateExecutor, error) {
	t.scopedMu.RLock()
	scopedTmplSet, ok := t.scopedTemplateSets[name]
	t.scopedMu.RUnlock()

	if !ok {
		var err error
		t.scopedMu.Lock()
		if scopedTmplSet, ok = t.scopedTemplateSets[name]; !ok {
			if scopedTmplSet, err = newScopedTemplateSet(t.all, name); err == nil {
				t.scopedTemplateSets[name] = scopedTmplSet
			}
		}
		t.scopedMu.Unlock()
		if err != nil {
			return nil, err
		}
	}

	if scopedTmplSet == nil {
		return nil, fmt.Errorf("template %s not found", name)
	}
	return scopedTmplSet.newExecutor(funcMap), nil
}

type scopedTemplateSet struct {
	name          string
	htmlTemplates map[string]*template.Template
	textTemplates map[string]*texttemplate.Template
	execFuncs     map[string]reflect.Value
}

func escapeTemplate(t *template.Template) error {
	// force the Golang HTML template to complete the escaping work
	err := t.Execute(io.Discard, nil)
	if _, ok := err.(*template.Error); ok {
		return err
	}
	return nil
}

//nolint:unused
type htmlTemplate struct {
	escapeErr error
	text      *texttemplate.Template
}

//nolint:unused
type textTemplateCommon struct {
	tmpl   map[string]*template.Template // Map from name to defined templates.
	muTmpl sync.RWMutex                  // protects tmpl
	option struct {
		missingKey int
	}
	muFuncs    sync.RWMutex // protects parseFuncs and execFuncs
	parseFuncs texttemplate.FuncMap
	execFuncs  map[string]reflect.Value
}

//nolint:unused
type textTemplate struct {
	name string
	*parse.Tree
	*textTemplateCommon
	leftDelim  string
	rightDelim string
}

func ptr[T, P any](ptr *P) *T {
	// https://pkg.go.dev/unsafe#Pointer
	// (1) Conversion of a *T1 to Pointer to *T2.
	// Provided that T2 is no larger than T1 and that the two share an equivalent memory layout,
	// this conversion allows reinterpreting data of one type as data of another type.
	return (*T)(unsafe.Pointer(ptr))
}

func newScopedTemplateSet(all *template.Template, name string) (*scopedTemplateSet, error) {
	targetTmpl := all.Lookup(name)
	if targetTmpl == nil {
		return nil, fmt.Errorf("template %q not found", name)
	}
	if err := escapeTemplate(targetTmpl); err != nil {
		return nil, fmt.Errorf("template %q has an error when escaping: %w", name, err)
	}

	ts := &scopedTemplateSet{
		name:          name,
		htmlTemplates: map[string]*template.Template{},
		textTemplates: map[string]*texttemplate.Template{},
	}

	htmlTmpl := ptr[htmlTemplate](all)
	textTmpl := htmlTmpl.text
	textTmplPtr := ptr[textTemplate](textTmpl)

	textTmplPtr.muFuncs.Lock()
	ts.execFuncs = map[string]reflect.Value{}
	for k, v := range textTmplPtr.execFuncs {
		ts.execFuncs[k] = v
	}
	textTmplPtr.muFuncs.Unlock()

	var collectTemplates func(nodes []parse.Node)
	var collectErr error // only need to collect the one error
	collectTemplates = func(nodes []parse.Node) {
		for _, node := range nodes {
			if node.Type() == parse.NodeTemplate {
				nodeTemplate := node.(*parse.TemplateNode)
				subName := nodeTemplate.Name
				if ts.htmlTemplates[subName] == nil {
					subTmpl := all.Lookup(subName)
					if subTmpl == nil {
						// HTML template will add some internal templates like "$delimDoubleQuote" into the text template
						ts.textTemplates[subName] = textTmpl.Lookup(subName)
					} else if subTmpl.Tree == nil || subTmpl.Tree.Root == nil {
						collectErr = fmt.Errorf("template %q has no tree, it's usually caused by broken templates", subName)
					} else {
						ts.htmlTemplates[subName] = subTmpl
						if err := escapeTemplate(subTmpl); err != nil {
							collectErr = fmt.Errorf("template %q has an error when escaping: %w", subName, err)
							return
						}
						collectTemplates(subTmpl.Tree.Root.Nodes)
					}
				}
			} else if node.Type() == parse.NodeList {
				nodeList := node.(*parse.ListNode)
				collectTemplates(nodeList.Nodes)
			} else if node.Type() == parse.NodeIf {
				nodeIf := node.(*parse.IfNode)
				collectTemplates(nodeIf.BranchNode.List.Nodes)
				if nodeIf.BranchNode.ElseList != nil {
					collectTemplates(nodeIf.BranchNode.ElseList.Nodes)
				}
			} else if node.Type() == parse.NodeRange {
				nodeRange := node.(*parse.RangeNode)
				collectTemplates(nodeRange.BranchNode.List.Nodes)
				if nodeRange.BranchNode.ElseList != nil {
					collectTemplates(nodeRange.BranchNode.ElseList.Nodes)
				}
			} else if node.Type() == parse.NodeWith {
				nodeWith := node.(*parse.WithNode)
				collectTemplates(nodeWith.BranchNode.List.Nodes)
				if nodeWith.BranchNode.ElseList != nil {
					collectTemplates(nodeWith.BranchNode.ElseList.Nodes)
				}
			}
		}
	}
	ts.htmlTemplates[name] = targetTmpl
	collectTemplates(targetTmpl.Tree.Root.Nodes)
	return ts, collectErr
}

func (ts *scopedTemplateSet) newExecutor(funcMap map[string]any) TemplateExecutor {
	tmpl := texttemplate.New("")
	tmplPtr := ptr[textTemplate](tmpl)
	tmplPtr.execFuncs = map[string]reflect.Value{}
	for k, v := range ts.execFuncs {
		tmplPtr.execFuncs[k] = v
	}
	if funcMap != nil {
		tmpl.Funcs(funcMap)
	}
	// after escapeTemplate, the html templates are also escaped text templates, so it could be added to the text template directly
	for _, t := range ts.htmlTemplates {
		_, _ = tmpl.AddParseTree(t.Name(), t.Tree)
	}
	for _, t := range ts.textTemplates {
		_, _ = tmpl.AddParseTree(t.Name(), t.Tree)
	}

	// now the text template has all necessary escaped templates, so we can safely execute, just like what the html template does
	return tmpl.Lookup(ts.name)
}
