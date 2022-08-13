// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import "strings"

type FormField struct {
	Type        string                 `yaml:"type"`
	Id          string                 `yaml:"id"`
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

// Valid checks whether an IssueFormTemplate is considered valid, e.g. at least name and about
func (it IssueFormTemplate) Valid() bool {
	if strings.TrimSpace(it.Name) == "" || strings.TrimSpace(it.About) == "" {
		return false
	}

	for _, field := range it.Fields {
		if strings.TrimSpace(field.Id) == "" {
			// TODO: add IDs should be optional, maybe generate slug from label? or use numberic id
			return false
		}
	}

	return true
}
