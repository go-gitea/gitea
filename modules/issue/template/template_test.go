// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package template

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		want     *api.IssueTemplate
		wantErr  string
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
			name: "dropdown invalid list",
			content: `
name: "test"
about: "this is about"
body:
  - type: "dropdown"
    id: "1"
    attributes:
      label: "a"
      list: "on"
`,
			wantErr: "body[0](dropdown): 'list' should be a bool",
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
		{
			name: "field is required but hidden",
			content: `
name: "test"
about: "this is about"
body:
  - type: "input"
    id: "1"
    attributes:
      label: "a"
    validations:
      required: true
    visible: [content]
`,
			wantErr: "body[0](input): can not require a hidden field",
		},
		{
			name: "checkboxes is required but hidden",
			content: `
name: "test"
about: "this is about"
body:
  - type: checkboxes
    id: "1"
    attributes:
      label: Label of checkboxes
      description: Description of checkboxes
      options:
        - label: Option 1
          required: false
        - label: Required and hidden
          required: true
          visible: [content]
`,
			wantErr: "body[0](checkboxes), option[1]: can not require a hidden checkbox",
		},
		{
			name: "dropdown default is not an integer",
			content: `
name: "test"
about: "this is about"
body:
  - type: dropdown
    id: "1"
    attributes:
      label: Label of dropdown
      description: Description of dropdown
      multiple: true
      options:
        - Option 1 of dropdown
        - Option 2 of dropdown
        - Option 3 of dropdown
      default: "def"
    validations:
      required: true
`,
			wantErr: "body[0](dropdown): 'default' should be an int",
		},
		{
			name: "dropdown default is out of range",
			content: `
name: "test"
about: "this is about"
body:
  - type: dropdown
    id: "1"
    attributes:
      label: Label of dropdown
      description: Description of dropdown
      multiple: true
      options:
        - Option 1 of dropdown
        - Option 2 of dropdown
        - Option 3 of dropdown
      default: 3
    validations:
      required: true
`,
			wantErr: "body[0](dropdown): the value of 'default' is out of range",
		},
		{
			name: "dropdown without default is valid",
			content: `
name: "test"
about: "this is about"
body:
  - type: dropdown
    id: "1"
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
`,
			want: &api.IssueTemplate{
				Name:  "test",
				About: "this is about",
				Fields: []*api.IssueFormField{
					{
						Type: "dropdown",
						ID:   "1",
						Attributes: map[string]any{
							"label":       "Label of dropdown",
							"description": "Description of dropdown",
							"multiple":    true,
							"options": []any{
								"Option 1 of dropdown",
								"Option 2 of dropdown",
								"Option 3 of dropdown",
							},
						},
						Validations: map[string]any{
							"required": true,
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm, api.IssueFormFieldVisibleContent},
					},
				},
				FileName: "test.yaml",
			},
			wantErr: "",
		},
		{
			name: "valid",
			content: `
name: Name
title: Title
about: About
labels: ["label1", "label2"]
assignees: ["user1", "user2"]
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
      default: 1
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
        - label: Hidden Option 3 of checkboxes
          visible: [content]
        - label: Required but not submitted
          required: true
          visible: [form]
`,
			want: &api.IssueTemplate{
				Name:      "Name",
				Title:     "Title",
				About:     "About",
				Labels:    []string{"label1", "label2"},
				Assignees: []string{"user1", "user2"},
				Ref:       "Ref",
				Fields: []*api.IssueFormField{
					{
						Type: "markdown",
						ID:   "id1",
						Attributes: map[string]any{
							"value": "Value of the markdown",
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm},
					},
					{
						Type: "textarea",
						ID:   "id2",
						Attributes: map[string]any{
							"label":       "Label of textarea",
							"description": "Description of textarea",
							"placeholder": "Placeholder of textarea",
							"value":       "Value of textarea",
							"render":      "bash",
						},
						Validations: map[string]any{
							"required": true,
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm, api.IssueFormFieldVisibleContent},
					},
					{
						Type: "input",
						ID:   "id3",
						Attributes: map[string]any{
							"label":       "Label of input",
							"description": "Description of input",
							"placeholder": "Placeholder of input",
							"value":       "Value of input",
						},
						Validations: map[string]any{
							"required":  true,
							"is_number": true,
							"regex":     "[a-zA-Z0-9]+",
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm, api.IssueFormFieldVisibleContent},
					},
					{
						Type: "dropdown",
						ID:   "id4",
						Attributes: map[string]any{
							"label":       "Label of dropdown",
							"description": "Description of dropdown",
							"multiple":    true,
							"options": []any{
								"Option 1 of dropdown",
								"Option 2 of dropdown",
								"Option 3 of dropdown",
							},
							"default": 1,
						},
						Validations: map[string]any{
							"required": true,
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm, api.IssueFormFieldVisibleContent},
					},
					{
						Type: "checkboxes",
						ID:   "id5",
						Attributes: map[string]any{
							"label":       "Label of checkboxes",
							"description": "Description of checkboxes",
							"options": []any{
								map[string]any{"label": "Option 1 of checkboxes", "required": true},
								map[string]any{"label": "Option 2 of checkboxes", "required": false},
								map[string]any{"label": "Hidden Option 3 of checkboxes", "visible": []string{"content"}},
								map[string]any{"label": "Required but not submitted", "required": true, "visible": []string{"form"}},
							},
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm, api.IssueFormFieldVisibleContent},
					},
				},
				FileName: "test.yaml",
			},
			wantErr: "",
		},
		{
			name: "single label",
			content: `
name: Name
title: Title
about: About
labels: label1
ref: Ref
body:
  - type: markdown
    id: id1
    attributes:
      value: Value of the markdown shown in form
  - type: markdown
    id: id2
    attributes:
      value: Value of the markdown shown in created issue
    visible: [content]
`,
			want: &api.IssueTemplate{
				Name:   "Name",
				Title:  "Title",
				About:  "About",
				Labels: []string{"label1"},
				Ref:    "Ref",
				Fields: []*api.IssueFormField{
					{
						Type: "markdown",
						ID:   "id1",
						Attributes: map[string]any{
							"value": "Value of the markdown shown in form",
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm},
					},
					{
						Type: "markdown",
						ID:   "id2",
						Attributes: map[string]any{
							"value": "Value of the markdown shown in created issue",
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleContent},
					},
				},
				FileName: "test.yaml",
			},
			wantErr: "",
		},
		{
			name: "comma-delimited labels",
			content: `
name: Name
title: Title
about: About
labels: label1,label2,,label3 ,,
ref: Ref
body:
  - type: markdown
    id: id1
    attributes:
      value: Value of the markdown
`,
			want: &api.IssueTemplate{
				Name:   "Name",
				Title:  "Title",
				About:  "About",
				Labels: []string{"label1", "label2", "label3"},
				Ref:    "Ref",
				Fields: []*api.IssueFormField{
					{
						Type: "markdown",
						ID:   "id1",
						Attributes: map[string]any{
							"value": "Value of the markdown",
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm},
					},
				},
				FileName: "test.yaml",
			},
			wantErr: "",
		},
		{
			name: "empty string as labels",
			content: `
name: Name
title: Title
about: About
labels: ''
ref: Ref
body:
  - type: markdown
    id: id1
    attributes:
      value: Value of the markdown
`,
			want: &api.IssueTemplate{
				Name:   "Name",
				Title:  "Title",
				About:  "About",
				Labels: nil,
				Ref:    "Ref",
				Fields: []*api.IssueFormField{
					{
						Type: "markdown",
						ID:   "id1",
						Attributes: map[string]any{
							"value": "Value of the markdown",
						},
						Visible: []api.IssueFormFieldVisible{api.IssueFormFieldVisibleForm},
					},
				},
				FileName: "test.yaml",
			},
			wantErr: "",
		},
		{
			name:     "comma delimited labels in markdown",
			filename: "test.md",
			content: `---
name: Name
title: Title
about: About
labels: label1,label2,,label3 ,,
ref: Ref
---
Content
`,
			want: &api.IssueTemplate{
				Name:     "Name",
				Title:    "Title",
				About:    "About",
				Labels:   []string{"label1", "label2", "label3"},
				Ref:      "Ref",
				Fields:   nil,
				Content:  "Content\n",
				FileName: "test.md",
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := "test.yaml"
			if tt.filename != "" {
				filename = tt.filename
			}
			tmpl, err := unmarshal(filename, []byte(tt.content))
			require.NoError(t, err)
			if tt.wantErr != "" {
				require.EqualError(t, Validate(tmpl), tt.wantErr)
			} else {
				require.NoError(t, Validate(tmpl))
				want, _ := json.Marshal(tt.want)
				got, _ := json.Marshal(tmpl)
				require.JSONEq(t, string(want), string(got))
			}
		})
	}
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
      value: Value of the markdown shown in form
  - type: markdown
    id: id2
    attributes:
      value: Value of the markdown shown in created issue
    visible: [content]
  - type: textarea
    id: id3
    attributes:
      label: Label of textarea
      description: Description of textarea
      placeholder: Placeholder of textarea
      value: Value of textarea
      render: bash
    validations:
      required: true
  - type: input
    id: id4
    attributes:
      label: Label of input
      description: Description of input
      placeholder: Placeholder of input
      value: Value of input
      hide_label: true
    validations:
      required: true
      is_number: true
      regex: "[a-zA-Z0-9]+"
  - type: dropdown
    id: id5
    attributes:
      label: Label of dropdown (one line)
      description: Description of dropdown
      multiple: true
      options:
        - Option 1 of dropdown
        - Option 2 of dropdown
        - Option 3 of dropdown
    validations:
      required: true
  - type: dropdown
    id: id6
    attributes:
      label: Label of dropdown (list)
      description: Description of dropdown
      multiple: true
      list: true
      options:
        - Option 1 of dropdown
        - Option 2 of dropdown
        - Option 3 of dropdown
    validations:
      required: true
  - type: checkboxes
    id: id7
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
          visible: [form]
        - label: Hidden Option of checkboxes
          visible: [content]
`,
				values: map[string][]string{
					"form-field-id3":   {"Value of id3"},
					"form-field-id4":   {"Value of id4"},
					"form-field-id5":   {"0,1"},
					"form-field-id6":   {"1,2"},
					"form-field-id7-0": {"on"},
					"form-field-id7-2": {"on"},
				},
			},

			want: `Value of the markdown shown in created issue

### Label of textarea

` + "```bash\nValue of id3\n```" + `

Value of id4

### Label of dropdown (one line)

Option 1 of dropdown, Option 2 of dropdown

### Label of dropdown (list)

- Option 2 of dropdown
- Option 3 of dropdown

### Label of checkboxes

- [x] Option 1 of checkboxes
- [ ] Option 2 of checkboxes
- [ ] Hidden Option of checkboxes

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
				assert.EqualValues(t, tt.want, got)
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
			got := minQuotes(tt.args.value)
			assert.Equal(t, tt.want, got)
		})
	}
}
