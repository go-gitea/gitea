// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// StateType issue state type
type StateType string

const (
	// StateOpen pr is opend
	StateOpen StateType = "open"
	// StateClosed pr is closed
	StateClosed StateType = "closed"
	// StateAll is all
	StateAll StateType = "all"
)

// PullRequestMeta PR info if an issue is a PR
type PullRequestMeta struct {
	HasMerged bool       `json:"merged"`
	Merged    *time.Time `json:"merged_at"`
}

// RepositoryMeta basic repository information
type RepositoryMeta struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	FullName string `json:"full_name"`
}

// Issue represents an issue in a repository
// swagger:model
type Issue struct {
	ID               int64      `json:"id"`
	URL              string     `json:"url"`
	HTMLURL          string     `json:"html_url"`
	Index            int64      `json:"number"`
	Poster           *User      `json:"user"`
	OriginalAuthor   string     `json:"original_author"`
	OriginalAuthorID int64      `json:"original_author_id"`
	Title            string     `json:"title"`
	Body             string     `json:"body"`
	Ref              string     `json:"ref"`
	Labels           []*Label   `json:"labels"`
	Milestone        *Milestone `json:"milestone"`
	// deprecated
	Assignee  *User   `json:"assignee"`
	Assignees []*User `json:"assignees"`
	// Whether the issue is open or closed
	//
	// type: string
	// enum: open,closed
	State    StateType `json:"state"`
	IsLocked bool      `json:"is_locked"`
	Comments int       `json:"comments"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`

	PullRequest *PullRequestMeta `json:"pull_request"`
	Repo        *RepositoryMeta  `json:"repository"`
}

// CreateIssueOption options to create one issue
type CreateIssueOption struct {
	// required:true
	Title string `json:"title" binding:"Required"`
	Body  string `json:"body"`
	Ref   string `json:"ref"`
	// deprecated
	Assignee  string   `json:"assignee"`
	Assignees []string `json:"assignees"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
	// milestone id
	Milestone int64 `json:"milestone"`
	// list of label ids
	Labels []int64 `json:"labels"`
	Closed bool    `json:"closed"`
}

// EditIssueOption options for editing an issue
type EditIssueOption struct {
	Title string  `json:"title"`
	Body  *string `json:"body"`
	Ref   *string `json:"ref"`
	// deprecated
	Assignee  *string  `json:"assignee"`
	Assignees []string `json:"assignees"`
	Milestone *int64   `json:"milestone"`
	State     *string  `json:"state"`
	// swagger:strfmt date-time
	Deadline       *time.Time `json:"due_date"`
	RemoveDeadline *bool      `json:"unset_due_date"`
}

// EditDeadlineOption options for creating a deadline
type EditDeadlineOption struct {
	// required:true
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}

// IssueDeadline represents an issue deadline
// swagger:model
type IssueDeadline struct {
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}

// IssueTemplate represents an issue template for a repository
// swagger:model
type IssueTemplate struct {
	Name     string                `json:"name" yaml:"name"`
	Title    string                `json:"title" yaml:"title"`
	About    string                `json:"about" yaml:"about"`
	Labels   []string              `json:"labels" yaml:"labels"`
	Ref      string                `json:"ref" yaml:"ref"`
	Content  string                `json:"content" yaml:"-"` // for markdown only
	Body     []*IssueTemplateField `json:"body" yaml:"body"` // for yaml only
	FileName string                `json:"file_name" yaml:"-"`
}

type IssueTemplateField struct {
	Type        string                 `json:"type" yaml:"type"`
	ID          string                 `json:"id" yaml:"id"`
	Attributes  map[string]interface{} `json:"attributes" yaml:"attributes"`
	Validations map[string]interface{} `json:"validations" yaml:"validations"`
}

func NewIssueTemplate(filename string, content []byte) (*IssueTemplate, error) {
	ret := &IssueTemplate{
		FileName: filename,
	}

	return ret, nil
}

// Validate checks whether an IssueTemplate is considered valid, and returns the first error
func (it *IssueTemplate) Validate() error {
	// TODO check the format of id, and more

	errMissField := func(f string) error {
		return fmt.Errorf("field '%s' is required", f)
	}

	if strings.TrimSpace(it.Name) == "" {
		return errMissField("name")
	}
	if strings.TrimSpace(it.About) == "" {
		return errMissField("about")
	}

	for idx, field := range it.Body {
		checkStringAttr := func(name string) error {
			attr := field.Attributes[name]
			if s, ok := attr.(string); !ok || s == "" {
				return fmt.Errorf(
					"body[%d]: the '%s' attribute is required and should be string with type %s",
					idx, name, field.Type,
				)
			}
			return nil
		}
		switch field.Type {
		case "markdown":
			if err := checkStringAttr("value"); err != nil {
				return err
			}
		case "textarea", "input", "dropdown":
			if err := checkStringAttr("label"); err != nil {
				return err
			}
		case "checkboxes":
			if err := checkStringAttr("label"); err != nil {
			}
			if err := checkStringAttr("label"); err != nil {
				return err
			}
			attr := field.Attributes["options"]
			if options, ok := attr.([]any); !ok {
				return fmt.Errorf(
					"body[%d]: the '%s' attribute is required and should be array with type %s",
					idx, "options", field.Type,
				)
			} else {
				for optIdx, option := range options {
					if opt, ok := option.(map[any]any); !ok {
						return fmt.Errorf(
							"body[%d], option[%d]: should be dictionary with type %s",
							idx, optIdx, field.Type,
						)
					} else {
						label := opt["label"]
						if s, ok := label.(string); !ok || s == "" {
							return fmt.Errorf(
								"body[%d], option[%d]: the '%s' is required and should be string with type %s",
								idx, optIdx, "label", field.Type,
							)
						}
					}
				}
			}

		default:
			return fmt.Errorf(
				"(field #%d '%s'): unknown type '%s'",
				idx+1, field.ID, field.Type,
			)
		}
	}
	return nil
}

// Valid checks whether an IssueTemplate is considered valid, e.g. at least name and about
func (it *IssueTemplate) Valid() bool {
	return it.Validate() == nil
}

// Type returns the type of IssueTemplate, it could be "md", "yaml" or empty for known
func (it *IssueTemplate) Type() string {
	if ext := filepath.Ext(it.FileName); ext == ".md" {
		return "md"
	} else if ext == ".yaml" || ext == ".yml" {
		return "yaml"
	}
	return ""
}

func (it *IssueTemplate) Fill(content []byte) error {
	if typ := it.Type(); typ == "md" {
		templateBody, err := it.extractMetadata(string(content), it)
		if err != nil {
			return fmt.Errorf("extract metadata: %w", err)
		}
		it.Content = templateBody
	} else if typ == "yaml" {
		if err := yaml.Unmarshal(content, it); err != nil {
			return fmt.Errorf("yaml unmarshal: %w", err)
		}
		if it.About == "" {
			// Compatible with treating description as about
			compatibleTemplate := &struct {
				About string `yaml:"description"`
			}{}
			if err := yaml.Unmarshal(content, compatibleTemplate); err == nil && compatibleTemplate.About != "" {
				it.About = compatibleTemplate.About
			}
		}
		for i, v := range it.Body {
			if v.ID == "" {
				v.ID = strconv.Itoa(i)
			}
		}
	}
	return nil
}

// extractMetadata consumes a markdown file, parses YAML frontmatter,
// and returns the frontmatter metadata separated from the markdown content.
// Copy from markdown.ExtractMetadata to avoid import cycle.
func (*IssueTemplate) extractMetadata(contents string, out interface{}) (string, error) {
	isYAMLSeparator := func(line string) bool {
		line = strings.TrimSpace(line)
		for i := 0; i < len(line); i++ {
			if line[i] != '-' {
				return false
			}
		}
		return len(line) > 2
	}

	var front, body []string
	lines := strings.Split(contents, "\n")
	for idx, line := range lines {
		if idx == 0 {
			// First line has to be a separator
			if !isYAMLSeparator(line) {
				return "", fmt.Errorf("frontmatter must start with a separator line")
			}
			continue
		}
		if isYAMLSeparator(line) {
			front, body = lines[1:idx], lines[idx+1:]
			break
		}
	}

	if len(front) == 0 {
		return "", fmt.Errorf("could not determine metadata")
	}

	if err := yaml.Unmarshal([]byte(strings.Join(front, "\n")), out); err != nil {
		return "", err
	}
	return strings.Join(body, "\n"), nil
}
