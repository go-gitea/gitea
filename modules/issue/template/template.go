// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package template

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/container"
	api "code.gitea.io/gitea/modules/structs"

	"gitea.com/go-chi/binding"
)

// Validate checks whether an IssueTemplate is considered valid, and returns the first error
func Validate(template *api.IssueTemplate) error {
	if err := validateMetadata(template); err != nil {
		return err
	}
	if template.Type() == api.IssueTemplateTypeYaml {
		if err := validateYaml(template); err != nil {
			return err
		}
	}
	return nil
}

func validateMetadata(template *api.IssueTemplate) error {
	if strings.TrimSpace(template.Name) == "" {
		return fmt.Errorf("'name' is required")
	}
	if strings.TrimSpace(template.About) == "" {
		return fmt.Errorf("'about' is required")
	}
	return nil
}

func validateYaml(template *api.IssueTemplate) error {
	if len(template.Fields) == 0 {
		return fmt.Errorf("'body' is required")
	}
	ids := make(container.Set[string])
	for idx, field := range template.Fields {
		if err := validateID(field, idx, ids); err != nil {
			return err
		}
		if err := validateLabel(field, idx); err != nil {
			return err
		}

		position := newErrorPosition(idx, field.Type)
		switch field.Type {
		case api.IssueFormFieldTypeMarkdown:
			if err := validateStringItem(position, field.Attributes, true, "value"); err != nil {
				return err
			}
		case api.IssueFormFieldTypeTextarea:
			if err := validateStringItem(position, field.Attributes, false,
				"description",
				"placeholder",
				"value",
				"render",
			); err != nil {
				return err
			}
		case api.IssueFormFieldTypeInput:
			if err := validateStringItem(position, field.Attributes, false,
				"description",
				"placeholder",
				"value",
			); err != nil {
				return err
			}
			if err := validateBoolItem(position, field.Validations, "is_number"); err != nil {
				return err
			}
			if err := validateStringItem(position, field.Validations, false, "regex"); err != nil {
				return err
			}
		case api.IssueFormFieldTypeDropdown:
			if err := validateStringItem(position, field.Attributes, false, "description"); err != nil {
				return err
			}
			if err := validateBoolItem(position, field.Attributes, "multiple"); err != nil {
				return err
			}
			if err := validateOptions(field, idx); err != nil {
				return err
			}
			if err := validateDropdownDefault(position, field.Attributes); err != nil {
				return err
			}
		case api.IssueFormFieldTypeCheckboxes:
			if err := validateStringItem(position, field.Attributes, false, "description"); err != nil {
				return err
			}
			if err := validateOptions(field, idx); err != nil {
				return err
			}
		default:
			return position.Errorf("unknown type")
		}

		if err := validateRequired(field, idx); err != nil {
			return err
		}
	}
	return nil
}

func validateLabel(field *api.IssueFormField, idx int) error {
	if field.Type == api.IssueFormFieldTypeMarkdown {
		// The label is not required for a markdown field
		return nil
	}
	return validateStringItem(newErrorPosition(idx, field.Type), field.Attributes, true, "label")
}

func validateRequired(field *api.IssueFormField, idx int) error {
	if field.Type == api.IssueFormFieldTypeMarkdown || field.Type == api.IssueFormFieldTypeCheckboxes {
		// The label is not required for a markdown or checkboxes field
		return nil
	}
	if err := validateBoolItem(newErrorPosition(idx, field.Type), field.Validations, "required"); err != nil {
		return err
	}
	if required, _ := field.Validations["required"].(bool); required && !field.VisibleOnForm() {
		return newErrorPosition(idx, field.Type).Errorf("can not require a hidden field")
	}
	return nil
}

func validateID(field *api.IssueFormField, idx int, ids container.Set[string]) error {
	if field.Type == api.IssueFormFieldTypeMarkdown {
		// The ID is not required for a markdown field
		return nil
	}

	position := newErrorPosition(idx, field.Type)
	if field.ID == "" {
		// If the ID is empty in yaml, template.Unmarshal will auto autofill it, so it cannot be empty
		return position.Errorf("'id' is required")
	}
	if binding.AlphaDashPattern.MatchString(field.ID) {
		return position.Errorf("'id' should contain only alphanumeric, '-' and '_'")
	}
	if !ids.Add(field.ID) {
		return position.Errorf("'id' should be unique")
	}
	return nil
}

func validateOptions(field *api.IssueFormField, idx int) error {
	if field.Type != api.IssueFormFieldTypeDropdown && field.Type != api.IssueFormFieldTypeCheckboxes {
		return nil
	}
	position := newErrorPosition(idx, field.Type)

	options, ok := field.Attributes["options"].([]any)
	if !ok || len(options) == 0 {
		return position.Errorf("'options' is required and should be a array")
	}

	for optIdx, option := range options {
		position := newErrorPosition(idx, field.Type, optIdx)
		switch field.Type {
		case api.IssueFormFieldTypeDropdown:
			if _, ok := option.(string); !ok {
				return position.Errorf("should be a string")
			}
		case api.IssueFormFieldTypeCheckboxes:
			opt, ok := option.(map[string]any)
			if !ok {
				return position.Errorf("should be a dictionary")
			}
			if label, ok := opt["label"].(string); !ok || label == "" {
				return position.Errorf("'label' is required and should be a string")
			}

			if visibility, ok := opt["visible"]; ok {
				visibilityList, ok := visibility.([]any)
				if !ok {
					return position.Errorf("'visible' should be list")
				}
				for _, visibleType := range visibilityList {
					visibleType, ok := visibleType.(string)
					if !ok || !(visibleType == "form" || visibleType == "content") {
						return position.Errorf("'visible' list can only contain strings of 'form' and 'content'")
					}
				}
			}

			if required, ok := opt["required"]; ok {
				if _, ok := required.(bool); !ok {
					return position.Errorf("'required' should be a bool")
				}

				// validate if hidden field is required
				if visibility, ok := opt["visible"]; ok {
					visibilityList, _ := visibility.([]any)
					isVisible := false
					for _, v := range visibilityList {
						if vv, _ := v.(string); vv == "form" {
							isVisible = true
							break
						}
					}
					if !isVisible {
						return position.Errorf("can not require a hidden checkbox")
					}
				}
			}
		}
	}
	return nil
}

func validateStringItem(position errorPosition, m map[string]any, required bool, names ...string) error {
	for _, name := range names {
		v, ok := m[name]
		if !ok {
			if required {
				return position.Errorf("'%s' is required", name)
			}
			return nil
		}
		attr, ok := v.(string)
		if !ok {
			return position.Errorf("'%s' should be a string", name)
		}
		if strings.TrimSpace(attr) == "" && required {
			return position.Errorf("'%s' is required", name)
		}
	}
	return nil
}

func validateBoolItem(position errorPosition, m map[string]any, names ...string) error {
	for _, name := range names {
		v, ok := m[name]
		if !ok {
			return nil
		}
		if _, ok := v.(bool); !ok {
			return position.Errorf("'%s' should be a bool", name)
		}
	}
	return nil
}

func validateDropdownDefault(position errorPosition, attributes map[string]any) error {
	v, ok := attributes["default"]
	if !ok {
		return nil
	}
	defaultValue, ok := v.(int)
	if !ok {
		return position.Errorf("'default' should be an int")
	}

	options, ok := attributes["options"].([]any)
	if !ok {
		// should not happen
		return position.Errorf("'options' is required and should be a array")
	}
	if defaultValue < 0 || defaultValue >= len(options) {
		return position.Errorf("the value of 'default' is out of range")
	}

	return nil
}

type errorPosition string

func (p errorPosition) Errorf(format string, a ...any) error {
	return fmt.Errorf(string(p)+": "+format, a...)
}

func newErrorPosition(fieldIdx int, fieldType api.IssueFormFieldType, optionIndex ...int) errorPosition {
	ret := fmt.Sprintf("body[%d](%s)", fieldIdx, fieldType)
	if len(optionIndex) > 0 {
		ret += fmt.Sprintf(", option[%d]", optionIndex[0])
	}
	return errorPosition(ret)
}

// RenderToMarkdown renders template to markdown with specified values
func RenderToMarkdown(template *api.IssueTemplate, values url.Values) string {
	builder := &strings.Builder{}

	for _, field := range template.Fields {
		f := &valuedField{
			IssueFormField: field,
			Values:         values,
		}
		if f.ID == "" || !f.VisibleInContent() {
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
	// write label
	if !f.HideLabel() {
		_, _ = fmt.Fprintf(builder, "### %s\n\n", f.Label())
	}

	blankPlaceholder := "_No response_\n"

	// write body
	switch f.Type {
	case api.IssueFormFieldTypeCheckboxes:
		for _, option := range f.Options() {
			if !option.VisibleInContent() {
				continue
			}
			checked := " "
			if option.IsChecked() {
				checked = "x"
			}
			_, _ = fmt.Fprintf(builder, "- [%s] %s\n", checked, option.Label())
		}
	case api.IssueFormFieldTypeDropdown:
		var checkeds []string
		for _, option := range f.Options() {
			if option.IsChecked() {
				checkeds = append(checkeds, option.Label())
			}
		}
		if len(checkeds) > 0 {
			_, _ = fmt.Fprintf(builder, "%s\n", strings.Join(checkeds, ", "))
		} else {
			_, _ = fmt.Fprint(builder, blankPlaceholder)
		}
	case api.IssueFormFieldTypeInput:
		if value := f.Value(); value == "" {
			_, _ = fmt.Fprint(builder, blankPlaceholder)
		} else {
			_, _ = fmt.Fprintf(builder, "%s\n", value)
		}
	case api.IssueFormFieldTypeTextarea:
		if value := f.Value(); value == "" {
			_, _ = fmt.Fprint(builder, blankPlaceholder)
		} else if render := f.Render(); render != "" {
			quotes := minQuotes(value)
			_, _ = fmt.Fprintf(builder, "%s%s\n%s\n%s\n", quotes, f.Render(), value, quotes)
		} else {
			_, _ = fmt.Fprintf(builder, "%s\n", value)
		}
	case api.IssueFormFieldTypeMarkdown:
		if value, ok := f.Attributes["value"].(string); ok {
			_, _ = fmt.Fprintf(builder, "%s\n", value)
		}
	}
	_, _ = fmt.Fprintln(builder)
}

func (f *valuedField) Label() string {
	if label, ok := f.Attributes["label"].(string); ok {
		return label
	}
	return ""
}

func (f *valuedField) HideLabel() bool {
	if f.Type == api.IssueFormFieldTypeMarkdown {
		return true
	}
	if label, ok := f.Attributes["hide_label"].(bool); ok {
		return label
	}
	return false
}

func (f *valuedField) Render() string {
	if render, ok := f.Attributes["render"].(string); ok {
		return render
	}
	return ""
}

func (f *valuedField) Value() string {
	return strings.TrimSpace(f.Get(fmt.Sprintf("form-field-" + f.ID)))
}

func (f *valuedField) Options() []*valuedOption {
	if options, ok := f.Attributes["options"].([]any); ok {
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
	data  any
	field *valuedField
}

func (o *valuedOption) Label() string {
	switch o.field.Type {
	case api.IssueFormFieldTypeDropdown:
		if label, ok := o.data.(string); ok {
			return label
		}
	case api.IssueFormFieldTypeCheckboxes:
		if vs, ok := o.data.(map[string]any); ok {
			if v, ok := vs["label"].(string); ok {
				return v
			}
		}
	}
	return ""
}

func (o *valuedOption) IsChecked() bool {
	switch o.field.Type {
	case api.IssueFormFieldTypeDropdown:
		checks := strings.Split(o.field.Get(fmt.Sprintf("form-field-%s", o.field.ID)), ",")
		idx := strconv.Itoa(o.index)
		for _, v := range checks {
			if v == idx {
				return true
			}
		}
		return false
	case api.IssueFormFieldTypeCheckboxes:
		return o.field.Get(fmt.Sprintf("form-field-%s-%d", o.field.ID, o.index)) == "on"
	}
	return false
}

func (o *valuedOption) VisibleInContent() bool {
	if o.field.Type == api.IssueFormFieldTypeCheckboxes {
		if vs, ok := o.data.(map[string]any); ok {
			if vl, ok := vs["visible"].([]any); ok {
				for _, v := range vl {
					if vv, _ := v.(string); vv == "content" {
						return true
					}
				}
				return false
			}
		}
	}
	return true
}

var minQuotesRegex = regexp.MustCompilePOSIX("^`{3,}")

// minQuotes return 3 or more back-quotes.
// If n back-quotes exists, use n+1 back-quotes to quote.
func minQuotes(value string) string {
	ret := "```"
	for _, v := range minQuotesRegex.FindAllString(value, -1) {
		if len(v) >= len(ret) {
			ret = v + "`"
		}
	}
	return ret
}
