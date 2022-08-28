// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"net/url"
	"strings"

	api "code.gitea.io/gitea/modules/structs"
)

func RenderToMarkdown(template *api.IssueTemplate, values url.Values) string {
	builder := &strings.Builder{}

	for _, field := range template.Fields {
		f := &valuedField{
			IssueFormField: field,
			Values:         values,
		}
		if f.ID == "" {
			continue
		}
		f.WriteTo(builder)
	}

	return builder.String()
}

type valuedField struct {
	*api.IssueFormField
	url.Values
}

func (f *valuedField) WriteTo(builder *strings.Builder) {
	if f.Type == "markdown" {
		// markdown blocks do not appear in output
		return
	}

	// write label
	_, _ = fmt.Fprintf(builder, "### %s\n", f.Label())

	// write body
	switch f.Type {
	case "checkboxes", "dropdown":
		for _, option := range f.Options() {
			checked := " "
			if option.IsChecked() {
				checked = "x"
			}
			_, _ = fmt.Fprintf(builder, "- [%s] %s\n", checked, option.Label())
		}
	case "input":
		_, _ = fmt.Fprintf(builder, "%s\n", f.Value())
	case "textarea":
		_, _ = fmt.Fprintf(builder, "```%s\n%s```", f.Render(), f.Value())
	}
	_, _ = fmt.Fprintln(builder)
}

func (f *valuedField) Label() string {
	if label, ok := f.Attributes["label"].(string); ok {
		return label
	}
	return ""
}

func (f *valuedField) Render() string {
	if render, ok := f.Attributes["render"].(string); ok {
		return render
	}
	return ""
}

func (f *valuedField) Value() string {
	return f.Get(fmt.Sprintf("form-field-" + f.ID))
}

func (f *valuedField) Options() []*valuedOption {
	if options, ok := f.Attributes["options"].([]interface{}); ok {
		ret := make([]*valuedOption, 0, len(options))
		for i, option := range options {
			ret = append(ret, &valuedOption{
				index: i,
				data:  option,
				field: f,
			})
		}
		return ret
	}
	return nil
}

type valuedOption struct {
	index int
	data  interface{}
	field *valuedField
}

func (o *valuedOption) Label() string {
	switch o.field.Type {
	case "dropdown":
		if label, ok := o.data.(string); ok {
			return label
		}
	case "checkboxes":
		if vs, ok := o.data.(map[interface{}]interface{}); ok {
			if v, ok := vs["label"].(string); ok {
				return v
			}
		}
	}
	return ""
}

func (o *valuedOption) IsChecked() bool {
	return o.field.Get(fmt.Sprintf("form-field-%s-%d", o.field.ID, o.index)) == "on"
}
