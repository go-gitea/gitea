// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "miss name",
			content: ``,
			wantErr: "'name' is required",
		},
		{
			name: "miss about",
			content: `
name: "test"
`,
			wantErr: "'about' is required",
		},
		{
			name: "miss body",
			content: `
name: "test"
about: "this is about"
`,
			wantErr: "'body' is required",
		},
		{
			name: "markdown miss value",
			content: `
name: "test"
about: "this is about"
body:
  - type: "markdown"
`,
			wantErr: "body[0](markdown): 'value' is required",
		},
		{
			name: "markdown invalid value",
			content: `
name: "test"
about: "this is about"
body:
  - type: "markdown"
    attributes:
      value: true
`,
			wantErr: "body[0](markdown): 'value' should be a string",
		},
		{
			name: "markdown empty value",
			content: `
name: "test"
about: "this is about"
body:
  - type: "markdown"
    attributes:
      value: ""
`,
			wantErr: "body[0](markdown): 'value' is required",
		},
		{
			name: "textarea invalid id",
			content: `
name: "test"
about: "this is about"
body:
  - type: "textarea"
    id: "?"
`,
			wantErr: "body[0](textarea): 'id' should contain only alphanumeric, '-' and '_'",
		},
		{
			name: "textarea miss label",
			content: `
name: "test"
about: "this is about"
body:
  - type: "textarea"
    id: "1"
`,
			wantErr: "body[0](textarea): 'label' is required",
		},
		{
			name: "textarea conflict id",
			content: `
name: "test"
about: "this is about"
body:
  - type: "textarea"
    id: "1"
    attributes:
      label: "a"
  - type: "textarea"
    id: "1"
    attributes:
      label: "b"
`,
			wantErr: "body[1](textarea): 'id' should be unique",
		},
		{
			name: "textarea invalid description",
			content: `
name: "test"
about: "this is about"
body:
  - type: "textarea"
    id: "1"
    attributes:
      label: "a"
      description: true
`,
			wantErr: "body[0](textarea): 'description' should be a string",
		},
		{
			name: "textarea invalid required",
			content: `
name: "test"
about: "this is about"
body:
  - type: "textarea"
    id: "1"
    attributes:
      label: "a"
    validations:
      required: "on"
`,
			wantErr: "body[0](textarea): 'required' should be a bool",
		},
		{
			name: "input invalid description",
			content: `
name: "test"
about: "this is about"
body:
  - type: "input"
    id: "1"
    attributes:
      label: "a"
      description: true
`,
			wantErr: "body[0](input): 'description' should be a string",
		},
		{
			name: "input invalid is_number",
			content: `
name: "test"
about: "this is about"
body:
  - type: "input"
    id: "1"
    attributes:
      label: "a"
    validations:
      is_number: "yes"
`,
			wantErr: "body[0](input): 'is_number' should be a bool",
		},
		{
			name: "input invalid regex",
			content: `
name: "test"
about: "this is about"
body:
  - type: "input"
    id: "1"
    attributes:
      label: "a"
    validations:
      regex: true
`,
			wantErr: "body[0](input): 'regex' should be a string",
		},
		{
			name: "dropdown invalid description",
			content: `
name: "test"
about: "this is about"
body:
  - type: "dropdown"
    id: "1"
    attributes:
      label: "a"
      description: true
`,
			wantErr: "body[0](dropdown): 'description' should be a string",
		},
		{
			name: "dropdown invalid multiple",
			content: `
name: "test"
about: "this is about"
body:
  - type: "dropdown"
    id: "1"
    attributes:
      label: "a"
      multiple: "on"
`,
			wantErr: "body[0](dropdown): 'multiple' should be a bool",
		},
		{
			name: "checkboxes invalid description",
			content: `
name: "test"
about: "this is about"
body:
  - type: "checkboxes"
    id: "1"
    attributes:
      label: "a"
      description: true
`,
			wantErr: "body[0](checkboxes): 'description' should be a string",
		},
		{
			name: "invalid type",
			content: `
name: "test"
about: "this is about"
body:
  - type: "video"
    id: "1"
    attributes:
      label: "a"
`,
			wantErr: "body[0](video): unknown type",
		},
		{
			name: "dropdown miss options",
			content: `
name: "test"
about: "this is about"
body:
  - type: "dropdown"
    id: "1"
    attributes:
      label: "a"
`,
			wantErr: "body[0](dropdown): 'options' is required and should be a array",
		},
		{
			name: "dropdown invalid options",
			content: `
name: "test"
about: "this is about"
body:
  - type: "dropdown"
    id: "1"
    attributes:
      label: "a"
      options:
        - "a"
        - true
`,
			wantErr: "body[0](dropdown), option[1]: should be a string",
		},
		{
			name: "checkboxes invalid options",
			content: `
name: "test"
about: "this is about"
body:
  - type: "checkboxes"
    id: "1"
    attributes:
      label: "a"
      options:
        - "a"
        - true
`,
			wantErr: "body[0](checkboxes), option[0]: should be a dictionary",
		},
		{
			name: "checkboxes option miss label",
			content: `
name: "test"
about: "this is about"
body:
  - type: "checkboxes"
    id: "1"
    attributes:
      label: "a"
      options:
        - required: true
`,
			wantErr: "body[0](checkboxes), option[0]: 'label' is required and should be a string",
		},
		{
			name: "checkboxes option invalid required",
			content: `
name: "test"
about: "this is about"
body:
  - type: "checkboxes"
    id: "1"
    attributes:
      label: "a"
      options:
        - label: "a"
          required: "on"
`,
			wantErr: "body[0](checkboxes), option[0]: 'required' should be a bool",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := unmarshal("test.yaml", []byte(tt.content))
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate(tmpl); (err == nil) != (tt.wantErr == "") || err != nil && err.Error() != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %q", err, tt.wantErr)
			}
		})
	}
}
