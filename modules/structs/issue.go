// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// StateType issue state type
type StateType string

const (
	// StateOpen pr is opened
	StateOpen StateType = "open"
	// StateClosed pr is closed
	StateClosed StateType = "closed"
	// StateAll is all
	StateAll StateType = "all"
)

// PullRequestMeta PR info if an issue is a PR
type PullRequestMeta struct {
	HasMerged        bool       `json:"merged"`
	Merged           *time.Time `json:"merged_at"`
	IsWorkInProgress bool       `json:"draft"`
	HTMLURL          string     `json:"html_url"`
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

	TimeEstimate int64 `json:"time_estimate"`

	PullRequest *PullRequestMeta `json:"pull_request"`
	Repo        *RepositoryMeta  `json:"repository"`

	PinOrder int `json:"pin_order"`
}

// CreateIssueOption options to create one issue
// swagger:model
type CreateIssueOption struct {
	// required:true
	// description: Title of the issue
	// example: Bug: API returns 500 error when creating issue
	Title string `json:"title" binding:"Required"`
	// description: Body of the issue (markdown supported)
	// example: When calling the POST /repos/{owner}/{repo}/issues endpoint, I get a 500 error.
	Body string `json:"body"`
	// description: Reference for the issue (e.g., a branch name or commit SHA)
	// example: main
	Ref string `json:"ref"`
	// deprecated: true
	// description: (Deprecated) Username of the assignee. Use assignees instead.
	Assignee string `json:"assignee"`
	// description: List of usernames to assign to the issue
	// example: ["username1", "username2"]
	Assignees []string `json:"assignees"`
	// swagger:strfmt date-time
	// description: Due date for the issue (only the date portion is used)
	// example: "2025-12-31T23:59:59Z"
	Deadline *time.Time `json:"due_date"`
	// description: Milestone ID to associate with the issue
	// example: 1
	Milestone int64 `json:"milestone"`
	// description: List of label IDs to associate with the issue
	// example: [1, 2, 3]
	Labels []int64 `json:"labels"`
	// description: Whether to create the issue as closed (default: false)
	Closed bool `json:"closed"`
}

// EditIssueOption options for editing an issue
// swagger:model
type EditIssueOption struct {
	// description: New title for the issue
	// example: Updated issue title
	Title string `json:"title"`
	// description: New body content for the issue (markdown supported). Set to empty string to clear.
	// example: Updated issue description with more details
	Body *string `json:"body"`
	// description: New reference for the issue
	Ref *string `json:"ref"`
	// deprecated: true
	// description: (Deprecated) Username of the assignee. Use assignees instead.
	Assignee *string `json:"assignee"`
	// description: List of usernames to assign to the issue. Replaces existing assignees.
	// example: ["username1", "username2"]
	Assignees []string `json:"assignees"`
	// description: Milestone ID to associate with the issue. Set to 0 to remove milestone.
	Milestone *int64 `json:"milestone"`
	// description: New state for the issue. Valid values: "open", "closed"
	// enum: [open, closed]
	// example: closed
	State *string `json:"state"`
	// swagger:strfmt date-time
	// description: New due date for the issue. Set to null to remove deadline.
	Deadline *time.Time `json:"due_date"`
	// description: Set to true to remove the due date
	RemoveDeadline *bool `json:"unset_due_date"`
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
	Type        IssueFormFieldType      `json:"type" yaml:"type"`
	ID          string                  `json:"id" yaml:"id"`
	Attributes  map[string]any          `json:"attributes" yaml:"attributes"`
	Validations map[string]any          `json:"validations" yaml:"validations"`
	Visible     []IssueFormFieldVisible `json:"visible,omitempty"`
}

func (iff IssueFormField) VisibleOnForm() bool {
	if len(iff.Visible) == 0 {
		return true
	}
	return slices.Contains(iff.Visible, IssueFormFieldVisibleForm)
}

func (iff IssueFormField) VisibleInContent() bool {
	if len(iff.Visible) == 0 {
		// we have our markdown exception
		return iff.Type != IssueFormFieldTypeMarkdown
	}
	return slices.Contains(iff.Visible, IssueFormFieldVisibleContent)
}

// IssueFormFieldVisible defines issue form field visible
// swagger:model
type IssueFormFieldVisible string

const (
	IssueFormFieldVisibleForm    IssueFormFieldVisible = "form"
	IssueFormFieldVisibleContent IssueFormFieldVisible = "content"
)

// IssueTemplate represents an issue template for a repository
// swagger:model
type IssueTemplate struct {
	Name      string                   `json:"name" yaml:"name"`
	Title     string                   `json:"title" yaml:"title"`
	About     string                   `json:"about" yaml:"about"` // Using "description" in a template file is compatible
	Labels    IssueTemplateStringSlice `json:"labels" yaml:"labels"`
	Assignees IssueTemplateStringSlice `json:"assignees" yaml:"assignees"`
	Ref       string                   `json:"ref" yaml:"ref"`
	Content   string                   `json:"content" yaml:"-"`
	Fields    []*IssueFormField        `json:"body" yaml:"body"`
	FileName  string                   `json:"file_name" yaml:"-"`
}

type IssueTemplateStringSlice []string

func (l *IssueTemplateStringSlice) UnmarshalYAML(value *yaml.Node) error {
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
		for v := range strings.SplitSeq(str, ",") {
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
	return fmt.Errorf("line %d: cannot unmarshal %s into IssueTemplateStringSlice", value.Line, value.ShortTag())
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
	Index int64 `json:"index"`
	// owner of the issue's repo
	Owner string `json:"owner"`
	Name  string `json:"repo"`
}

// LockIssueOption options to lock an issue
type LockIssueOption struct {
	Reason string `json:"lock_reason"`
}
