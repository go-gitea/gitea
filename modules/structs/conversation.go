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

// Conversation represents an conversation in a repository
// swagger:model
type Conversation struct {
	ID      int64  `json:"id"`
	URL     string `json:"url"`
	HTMLURL string `json:"html_url"`
	Index   int64  `json:"number"`
	Ref     string `json:"ref"`
	// Whether the conversation is open or locked
	//
	// type: string
	// enum: open,locked
	State    StateType `json:"state"`
	IsLocked bool      `json:"is_locked"`
	Comments int       `json:"comments"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Locked *time.Time `json:"locked_at"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`

	Repo *RepositoryMeta `json:"repository"`
}

// CreateConversationOption options to create one conversation
type CreateConversationOption struct {
	Locked bool `json:"locked"`
}

// ConversationFormFieldType defines conversation form field type, can be "markdown", "textarea", "input", "dropdown" or "checkboxes"
type ConversationFormFieldType string

const (
	ConversationFormFieldTypeMarkdown   ConversationFormFieldType = "markdown"
	ConversationFormFieldTypeTextarea   ConversationFormFieldType = "textarea"
	ConversationFormFieldTypeInput      ConversationFormFieldType = "input"
	ConversationFormFieldTypeDropdown   ConversationFormFieldType = "dropdown"
	ConversationFormFieldTypeCheckboxes ConversationFormFieldType = "checkboxes"
)

// ConversationFormField represents a form field
// swagger:model
type ConversationFormField struct {
	Type        ConversationFormFieldType      `json:"type" yaml:"type"`
	ID          string                         `json:"id" yaml:"id"`
	Attributes  map[string]any                 `json:"attributes" yaml:"attributes"`
	Validations map[string]any                 `json:"validations" yaml:"validations"`
	Visible     []ConversationFormFieldVisible `json:"visible,omitempty"`
}

func (iff ConversationFormField) VisibleOnForm() bool {
	if len(iff.Visible) == 0 {
		return true
	}
	return slices.Contains(iff.Visible, ConversationFormFieldVisibleForm)
}

func (iff ConversationFormField) VisibleInContent() bool {
	if len(iff.Visible) == 0 {
		// we have our markdown exception
		return iff.Type != ConversationFormFieldTypeMarkdown
	}
	return slices.Contains(iff.Visible, ConversationFormFieldVisibleContent)
}

// ConversationFormFieldVisible defines conversation form field visible
// swagger:model
type ConversationFormFieldVisible string

const (
	ConversationFormFieldVisibleForm    ConversationFormFieldVisible = "form"
	ConversationFormFieldVisibleContent ConversationFormFieldVisible = "content"
)

// ConversationTemplate represents an conversation template for a repository
// swagger:model
type ConversationTemplate struct {
	Name      string                          `json:"name" yaml:"name"`
	Title     string                          `json:"title" yaml:"title"`
	About     string                          `json:"about" yaml:"about"` // Using "description" in a template file is compatible
	Labels    ConversationTemplateStringSlice `json:"labels" yaml:"labels"`
	Assignees ConversationTemplateStringSlice `json:"assignees" yaml:"assignees"`
	Ref       string                          `json:"ref" yaml:"ref"`
	Content   string                          `json:"content" yaml:"-"`
	Fields    []*ConversationFormField        `json:"body" yaml:"body"`
	FileName  string                          `json:"file_name" yaml:"-"`
}

type ConversationTemplateStringSlice []string

func (l *ConversationTemplateStringSlice) UnmarshalYAML(value *yaml.Node) error {
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
	return fmt.Errorf("line %d: cannot unmarshal %s into ConversationTemplateStringSlice", value.Line, value.ShortTag())
}

type ConversationConfigContactLink struct {
	Name  string `json:"name" yaml:"name"`
	URL   string `json:"url" yaml:"url"`
	About string `json:"about" yaml:"about"`
}

type ConversationConfig struct {
	BlankConversationsEnabled bool                            `json:"blank_conversations_enabled" yaml:"blank_conversations_enabled"`
	ContactLinks              []ConversationConfigContactLink `json:"contact_links" yaml:"contact_links"`
}

type ConversationConfigValidation struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ConversationTemplateType defines conversation template type
type ConversationTemplateType string

const (
	ConversationTemplateTypeMarkdown ConversationTemplateType = "md"
	ConversationTemplateTypeYaml     ConversationTemplateType = "yaml"
)

// Type returns the type of ConversationTemplate, can be "md", "yaml" or empty for known
func (it ConversationTemplate) Type() ConversationTemplateType {
	if base := path.Base(it.FileName); base == "config.yaml" || base == "config.yml" {
		// ignore config.yaml which is a special configuration file
		return ""
	}
	if ext := path.Ext(it.FileName); ext == ".md" {
		return ConversationTemplateTypeMarkdown
	} else if ext == ".yaml" || ext == ".yml" {
		return ConversationTemplateTypeYaml
	}
	return ""
}

// ConversationMeta basic conversation information
// swagger:model
type ConversationMeta struct {
	Index int64  `json:"index"`
	Owner string `json:"owner"`
	Name  string `json:"repo"`
}
