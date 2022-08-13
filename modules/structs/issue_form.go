// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"fmt"
	"strings"
)

type FormField struct {
	Type        string                 `yaml:"type"`
	ID          string                 `yaml:"id"`
	Attributes  map[string]interface{} `yaml:"attributes"`
	Validations map[string]interface{} `yaml:"validations"`
}

// IssueFormTemplate represents an issue form template for a repository
// swagger:model
type IssueFormTemplate struct {
	Name      string      `yaml:"name"`
	Title     string      `yaml:"title"`
	About     string      `yaml:"description"`
	Labels    []string    `yaml:"labels"`
	Assignees []string    `yaml:"assignees"`
	Ref       string      `yaml:"ref"`
	Fields    []FormField `yaml:"body"`
	FileName  string      `yaml:"-"`
}

// Valid checks whether an IssueFormTemplate is considered valid, e.g. at least name and about, and labels for all fields
func (it IssueFormTemplate) Valid() []string {
	// TODO: Localize error messages
	// TODO: Add a bunch more validations
	var errs []string

	if strings.TrimSpace(it.Name) == "" {
		errs = append(errs, "the 'name' field of the issue template are required")
	}
	if strings.TrimSpace(it.About) == "" {
		errs = append(errs, "the 'about' field of the issue template are required")
	}

	// Make sure all non-markdown fields have labels
	for fieldIdx, field := range it.Fields {
		// Make checker functions
		checkStringAttr := func(attrName string) {
			attr := field.Attributes[attrName]
			if attr == nil || strings.TrimSpace(attr.(string)) == "" {
				errs = append(errs, fmt.Sprintf(
					"(field #%d '%s'): the '%s' attribute is required for fields with type %s",
					fieldIdx+1, field.ID, attrName, field.Type,
				))
			}
		}
		checkOptionsStringAttr := func(optionIdx int, option map[interface{}]interface{}, attrName string) {
			attr := option[attrName]
			if attr == nil || strings.TrimSpace(attr.(string)) == "" {
				errs = append(errs, fmt.Sprintf(
					"(field #%d '%s', option #%d): the '%s' field is required for options",
					fieldIdx+1, field.ID, optionIdx, attrName,
				))
			}
		}
		checkListAttr := func(attrName string, itemChecker func(int, map[interface{}]interface{})) {
			attr := field.Attributes[attrName]
			if attr == nil {
				errs = append(errs, fmt.Sprintf(
					"(field #%d '%s'): the '%s' attribute is required for fields with type %s",
					fieldIdx+1, field.ID, attrName, field.Type,
				))
			} else {
				for i, item := range attr.([]interface{}) {
					itemChecker(i, item.(map[interface{}]interface{}))
				}
			}
		}

		// Make sure each field has its attributes
		switch field.Type {
		case "markdown":
			checkStringAttr("value")
		case "textarea", "input", "dropdown":
			checkStringAttr("label")
		case "checkboxes":
			checkStringAttr("label")
			checkListAttr("options", func(i int, item map[interface{}]interface{}) {
				checkOptionsStringAttr(i, item, "label")
			})
		default:
			errs = append(errs, fmt.Sprintf(
				"(field #%d '%s'): unknown type '%s'",
				fieldIdx+1, field.ID, field.Type,
			))
		}
	}

	return errs
}
