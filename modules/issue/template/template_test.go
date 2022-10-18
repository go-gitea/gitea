// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package template

import (
	"net/url"
	"reflect"
	"testing"

	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
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

	t.Run("valid", func(t *testing.T) {
		content := `
name: Name
title: Title
about: About
labels: ["label1", "label2"]
ref: Ref
body:
  - type: markdown
    id: id1
    attributes:
      value: Value of the markdown
  - type: textarea
    id: id2
    attributes:
      label: Label of textarea
      description: Description of textarea
      placeholder: Placeholder of textarea
      value: Value of textarea
      render: bash
    validations:
      required: true
  - type: input
    id: id3
    attributes:
      label: Label of input
      description: Description of input
      placeholder: Placeholder of input
      value: Value of input
    validations:
      required: true
      is_number: true
      regex: "[a-zA-Z0-9]+"
  - type: dropdown
    id: id4
    attributes:
      label: Label of dropdown
      description: Description of dropdown
      multiple: true
      options:
        - Option 1 of dropdown
        - Option 2 of dropdown
        - Option 3 of dropdown
    validations:
      required: true
  - type: checkboxes
    id: id5
    attributes:
      label: Label of checkboxes
      description: Description of checkboxes
      options:
        - label: Option 1 of checkboxes
          required: true
        - label: Option 2 of checkboxes
          required: false
        - label: Option 3 of checkboxes
          required: true
`
		want := &api.IssueTemplate{
			Name:   "Name",
			Title:  "Title",
			About:  "About",
			Labels: []string{"label1", "label2"},
			Ref:    "Ref",
			Fields: []*api.IssueFormField{
				{
					Type: "markdown",
					ID:   "id1",
					Attributes: map[string]interface{}{
						"value": "Value of the markdown",
					},
				},
				{
					Type: "textarea",
					ID:   "id2",
					Attributes: map[string]interface{}{
						"label":       "Label of textarea",
						"description": "Description of textarea",
						"placeholder": "Placeholder of textarea",
						"value":       "Value of textarea",
						"render":      "bash",
					},
					Validations: map[string]interface{}{
						"required": true,
					},
				},
				{
					Type: "input",
					ID:   "id3",
					Attributes: map[string]interface{}{
						"label":       "Label of input",
						"description": "Description of input",
						"placeholder": "Placeholder of input",
						"value":       "Value of input",
					},
					Validations: map[string]interface{}{
						"required":  true,
						"is_number": true,
						"regex":     "[a-zA-Z0-9]+",
					},
				},
				{
					Type: "dropdown",
					ID:   "id4",
					Attributes: map[string]interface{}{
						"label":       "Label of dropdown",
						"description": "Description of dropdown",
						"multiple":    true,
						"options": []interface{}{
							"Option 1 of dropdown",
							"Option 2 of dropdown",
							"Option 3 of dropdown",
						},
					},
					Validations: map[string]interface{}{
						"required": true,
					},
				},
				{
					Type: "checkboxes",
					ID:   "id5",
					Attributes: map[string]interface{}{
						"label":       "Label of checkboxes",
						"description": "Description of checkboxes",
						"options": []interface{}{
							map[interface{}]interface{}{"label": "Option 1 of checkboxes", "required": true},
							map[interface{}]interface{}{"label": "Option 2 of checkboxes", "required": false},
							map[interface{}]interface{}{"label": "Option 3 of checkboxes", "required": true},
						},
					},
				},
			},
			FileName: "test.yaml",
		}
		got, err := unmarshal("test.yaml", []byte(content))
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(got); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
		if !reflect.DeepEqual(want, got) {
			jsonWant, _ := json.Marshal(want)
			jsonGot, _ := json.Marshal(got)
			t.Errorf("want:\n%s\ngot:\n%s", jsonWant, jsonGot)
		}
	})
}

func TestRenderToMarkdown(t *testing.T) {
	type args struct {
		template string
		values   url.Values
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "normal",
			args: args{
				template: `
name: Name
title: Title
about: About
labels: ["label1", "label2"]
ref: Ref
body:
  - type: markdown
    id: id1
    attributes:
      value: Value of the markdown
  - type: textarea
    id: id2
    attributes:
      label: Label of textarea
      description: Description of textarea
      placeholder: Placeholder of textarea
      value: Value of textarea
      render: bash
    validations:
      required: true
  - type: input
    id: id3
    attributes:
      label: Label of input
      description: Description of input
      placeholder: Placeholder of input
      value: Value of input
    validations:
      required: true
      is_number: true
      regex: "[a-zA-Z0-9]+"
  - type: dropdown
    id: id4
    attributes:
      label: Label of dropdown
      description: Description of dropdown
      multiple: true
      options:
        - Option 1 of dropdown
        - Option 2 of dropdown
        - Option 3 of dropdown
    validations:
      required: true
  - type: checkboxes
    id: id5
    attributes:
      label: Label of checkboxes
      description: Description of checkboxes
      options:
        - label: Option 1 of checkboxes
          required: true
        - label: Option 2 of checkboxes
          required: false
        - label: Option 3 of checkboxes
          required: true
`,
				values: map[string][]string{
					"form-field-id2":   {"Value of id2"},
					"form-field-id3":   {"Value of id3"},
					"form-field-id4":   {"0,1"},
					"form-field-id5-0": {"on"},
					"form-field-id5-2": {"on"},
				},
			},
			want: `### Label of textarea

` + "```bash\nValue of id2\n```" + `

### Label of input

Value of id3

### Label of dropdown

Option 1 of dropdown, Option 2 of dropdown

### Label of checkboxes

- [x] Option 1 of checkboxes
- [ ] Option 2 of checkboxes
- [x] Option 3 of checkboxes

`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template, err := Unmarshal("test.yaml", []byte(tt.args.template))
			if err != nil {
				t.Fatal(err)
			}
			if got := RenderToMarkdown(template, tt.args.values); got != tt.want {
				t.Errorf("RenderToMarkdown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_minQuotes(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "without quote",
			args: args{
				value: "Hello\nWorld",
			},
			want: "```",
		},
		{
			name: "with 1 quote",
			args: args{
				value: "Hello\nWorld\n`text`\n",
			},
			want: "```",
		},
		{
			name: "with 3 quotes",
			args: args{
				value: "Hello\nWorld\n`text`\n```go\ntext\n```\n",
			},
			want: "````",
		},
		{
			name: "with more quotes",
			args: args{
				value: "Hello\nWorld\n`text`\n```go\ntext\n```\n``````````bash\ntext\n``````````\n",
			},
			want: "```````````",
		},
		{
			name: "not leading quotes",
			args: args{
				value: "Hello\nWorld`text````go\ntext`````````````bash\ntext``````````\n",
			},
			want: "```",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := minQuotes(tt.args.value); got != tt.want {
				t.Errorf("minQuotes() = %v, want %v", got, tt.want)
			}
		})
	}
}
