// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"fmt"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
	ID               int64         `json:"id"`
	URL              string        `json:"url"`
	HTMLURL          string        `json:"html_url"`
	Index            int64         `json:"number"`
	Poster           *User         `json:"user"`
	OriginalAuthor   string        `json:"original_author"`
	OriginalAuthorID int64         `json:"original_author_id"`
	Title            string        `json:"title"`
	Body             string        `json:"body"`
	Ref              string        `json:"ref"`
	Attachments      []*Attachment `json:"assets"`
	Labels           []*Label      `json:"labels"`
	Milestone        *Milestone    `json:"milestone"`
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

// IssueFormFieldType defines issue form field type, can be "markdown", "textarea", "input", "dropdown" or "checkboxes"
type IssueFormFieldType string

const (
	IssueFormFieldTypeMarkdown   IssueFormFieldType = "markdown"
	IssueFormFieldTypeTextarea   IssueFormFieldType = "textarea"
	IssueFormFieldTypeInput      IssueFormFieldType = "input"
	IssueFormFieldTypeDropdown   IssueFormFieldType = "dropdown"
	IssueFormFieldTypeCheckboxes IssueFormFieldType = "checkboxes"
)

// IssueFormField represents a form field
// swagger:model
type IssueFormField struct {
	Type        IssueFormFieldType     `json:"type" yaml:"type"`
	ID          string                 `json:"id" yaml:"id"`
	Attributes  map[string]interface{} `json:"attributes" yaml:"attributes"`
	Validations map[string]interface{} `json:"validations" yaml:"validations"`
}

// IssueTemplate represents an issue template for a repository
// swagger:model
type IssueTemplate struct {
	Name     string              `json:"name" yaml:"name"`
	Title    string              `json:"title" yaml:"title"`
	About    string              `json:"about" yaml:"about"` // Using "description" in a template file is compatible
	Labels   IssueTemplateLabels `json:"labels" yaml:"labels"`
	Ref      string              `json:"ref" yaml:"ref"`
	Content  string              `json:"content" yaml:"-"`
	Fields   []*IssueFormField   `json:"body" yaml:"body"`
	FileName string              `json:"file_name" yaml:"-"`
}

type IssueTemplateLabels []string

func (l *IssueTemplateLabels) UnmarshalYAML(value *yaml.Node) error {
	var labels []string
	if value.IsZero() {
		*l = labels
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		str := ""
		err := value.Decode(&str)
		if err != nil {
			return err
		}
		for _, v := range strings.Split(str, ",") {
			if v = strings.TrimSpace(v); v == "" {
				continue
			}
			labels = append(labels, v)
		}
		*l = labels
		return nil
	case yaml.SequenceNode:
		if err := value.Decode(&labels); err != nil {
			return err
		}
		*l = labels
		return nil
	}
	return fmt.Errorf("line %d: cannot unmarshal %s into IssueTemplateLabels", value.Line, value.ShortTag())
}

type IssueConfigContactLink struct {
	Name  string `json:"name" yaml:"name"`
	URL   string `json:"url" yaml:"url"`
	About string `json:"about" yaml:"about"`
}

type IssueConfig struct {
	BlankIssuesEnabled bool                     `json:"blank_issues_enabled" yaml:"blank_issues_enabled"`
	ContactLinks       []IssueConfigContactLink `json:"contact_links" yaml:"contact_links"`
}

type IssueConfigValidation struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// IssueTemplateType defines issue template type
type IssueTemplateType string

const (
	IssueTemplateTypeMarkdown IssueTemplateType = "md"
	IssueTemplateTypeYaml     IssueTemplateType = "yaml"
)

// Type returns the type of IssueTemplate, can be "md", "yaml" or empty for known
func (it IssueTemplate) Type() IssueTemplateType {
	if base := path.Base(it.FileName); base == "config.yaml" || base == "config.yml" {
		// ignore config.yaml which is a special configuration file
		return ""
	}
	if ext := path.Ext(it.FileName); ext == ".md" {
		return IssueTemplateTypeMarkdown
	} else if ext == ".yaml" || ext == ".yml" {
		return IssueTemplateTypeYaml
	}
	return ""
}

// IssueMeta basic issue information
// swagger:model
type IssueMeta struct {
	Index int64  `json:"index"`
	Owner string `json:"owner"`
	Name  string `json:"repo"`
}
