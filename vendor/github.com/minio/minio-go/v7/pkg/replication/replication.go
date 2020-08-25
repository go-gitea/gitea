/*
 * MinIO Client (C) 2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package replication

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rs/xid"
)

var errInvalidFilter = fmt.Errorf("Invalid filter")

// OptionType specifies operation to be performed on config
type OptionType string

const (
	// AddOption specifies addition of rule to config
	AddOption OptionType = "Add"
	// SetOption specifies modification of existing rule to config
	SetOption OptionType = "Set"

	// RemoveOption specifies rule options are for removing a rule
	RemoveOption OptionType = "Remove"
	// ImportOption is for getting current config
	ImportOption OptionType = "Import"
)

// Options represents options to set a replication configuration rule
type Options struct {
	Op           OptionType
	ID           string
	Prefix       string
	RuleStatus   string
	Priority     string
	TagString    string
	StorageClass string
	Arn          string
}

// Tags returns a slice of tags for a rule
func (opts Options) Tags() []Tag {
	var tagList []Tag
	tagTokens := strings.Split(opts.TagString, "&")
	for _, tok := range tagTokens {
		if tok == "" {
			break
		}
		kv := strings.SplitN(tok, "=", 2)
		tagList = append(tagList, Tag{
			Key:   kv[0],
			Value: kv[1],
		})
	}
	return tagList
}

// Config - replication configuration specified in
// https://docs.aws.amazon.com/AmazonS3/latest/dev/replication-add-config.html
type Config struct {
	XMLName xml.Name `xml:"ReplicationConfiguration" json:"-"`
	Rules   []Rule   `xml:"Rule" json:"Rules"`
	Role    string   `xml:"Role" json:"Role"`
}

// Empty returns true if config is not set
func (c *Config) Empty() bool {
	return len(c.Rules) == 0
}

// AddRule adds a new rule to existing replication config. If a rule exists with the
// same ID, then the rule is replaced.
func (c *Config) AddRule(opts Options) error {
	tags := opts.Tags()
	andVal := And{
		Tags: opts.Tags(),
	}
	filter := Filter{Prefix: opts.Prefix}
	// only a single tag is set.
	if opts.Prefix == "" && len(tags) == 1 {
		filter.Tag = tags[0]
	}
	// both prefix and tag are present
	if len(andVal.Tags) > 1 || opts.Prefix != "" {
		filter.And = andVal
		filter.And.Prefix = opts.Prefix
		filter.Prefix = ""
	}
	if opts.ID == "" {
		opts.ID = xid.New().String()
	}
	var status Status
	// toggle rule status for edit option
	switch opts.RuleStatus {
	case "enable":
		status = Enabled
	case "disable":
		status = Disabled
	}
	arnStr := opts.Arn
	if opts.Arn == "" {
		arnStr = c.Role
	}
	tokens := strings.Split(arnStr, ":")
	if len(tokens) != 6 {
		return fmt.Errorf("invalid format for replication Arn")
	}
	if c.Role == "" { // for new configurations
		c.Role = opts.Arn
	}
	priority, err := strconv.Atoi(opts.Priority)
	if err != nil {
		return err
	}
	newRule := Rule{
		ID:       opts.ID,
		Priority: priority,
		Status:   status,
		Filter:   filter,
		Destination: Destination{
			Bucket:       fmt.Sprintf("arn:aws:s3:::%s", tokens[5]),
			StorageClass: opts.StorageClass,
		},
		DeleteMarkerReplication: DeleteMarkerReplication{Status: Disabled},
	}

	ruleFound := false
	for i, rule := range c.Rules {
		if rule.Priority == newRule.Priority && rule.ID != newRule.ID {
			return fmt.Errorf("Priority must be unique. Replication configuration already has a rule with this priority")
		}
		if rule.Destination.Bucket != newRule.Destination.Bucket {
			return fmt.Errorf("The destination bucket must be same for all rules")
		}
		if rule.ID != newRule.ID {
			continue
		}
		if opts.Priority == "" && rule.ID == newRule.ID {
			// inherit priority from existing rule, required field on server
			newRule.Priority = rule.Priority
		}
		if opts.RuleStatus == "" {
			newRule.Status = rule.Status
		}
		c.Rules[i] = newRule
		ruleFound = true
		break
	}
	// validate rule after overlaying priority for pre-existing rule being disabled.
	if err := newRule.Validate(); err != nil {
		return err
	}
	if !ruleFound && opts.Op == SetOption {
		return fmt.Errorf("Rule with ID %s not found in replication configuration", opts.ID)
	}
	if !ruleFound {
		c.Rules = append(c.Rules, newRule)
	}
	return nil
}

// RemoveRule removes a rule from replication config.
func (c *Config) RemoveRule(opts Options) error {
	var newRules []Rule
	for _, rule := range c.Rules {
		if rule.ID != opts.ID {
			newRules = append(newRules, rule)
		}
	}

	if len(newRules) == 0 {
		return fmt.Errorf("Replication configuration should have at least one rule")
	}
	c.Rules = newRules
	return nil

}

// Rule - a rule for replication configuration.
type Rule struct {
	XMLName                 xml.Name                `xml:"Rule" json:"-"`
	ID                      string                  `xml:"ID,omitempty"`
	Status                  Status                  `xml:"Status"`
	Priority                int                     `xml:"Priority"`
	DeleteMarkerReplication DeleteMarkerReplication `xml:"DeleteMarkerReplication"`
	Destination             Destination             `xml:"Destination"`
	Filter                  Filter                  `xml:"Filter" json:"Filter"`
}

// Validate validates the rule for correctness
func (r Rule) Validate() error {
	if err := r.validateID(); err != nil {
		return err
	}
	if err := r.validateStatus(); err != nil {
		return err
	}
	if err := r.validateFilter(); err != nil {
		return err
	}

	if r.Priority < 0 && r.Status == Enabled {
		return fmt.Errorf("Priority must be set for the rule")
	}

	return nil
}

// validateID - checks if ID is valid or not.
func (r Rule) validateID() error {
	// cannot be longer than 255 characters
	if len(r.ID) > 255 {
		return fmt.Errorf("ID must be less than 255 characters")
	}
	return nil
}

// validateStatus - checks if status is valid or not.
func (r Rule) validateStatus() error {
	// Status can't be empty
	if len(r.Status) == 0 {
		return fmt.Errorf("status cannot be empty")
	}

	// Status must be one of Enabled or Disabled
	if r.Status != Enabled && r.Status != Disabled {
		return fmt.Errorf("status must be set to either Enabled or Disabled")
	}
	return nil
}

func (r Rule) validateFilter() error {
	if err := r.Filter.Validate(); err != nil {
		return err
	}
	return nil
}

// Prefix - a rule can either have prefix under <filter></filter> or under
// <filter><and></and></filter>. This method returns the prefix from the
// location where it is available
func (r Rule) Prefix() string {
	if r.Filter.Prefix != "" {
		return r.Filter.Prefix
	}
	return r.Filter.And.Prefix
}

// Tags - a rule can either have tag under <filter></filter> or under
// <filter><and></and></filter>. This method returns all the tags from the
// rule in the format tag1=value1&tag2=value2
func (r Rule) Tags() string {
	if len(r.Filter.And.Tags) != 0 {
		var buf bytes.Buffer
		for _, t := range r.Filter.And.Tags {
			if buf.Len() > 0 {
				buf.WriteString("&")
			}
			buf.WriteString(t.String())
		}
		return buf.String()
	}
	return ""
}

// Filter - a filter for a replication configuration Rule.
type Filter struct {
	XMLName xml.Name `xml:"Filter" json:"-"`
	Prefix  string   `json:"Prefix,omitempty"`
	And     And      `xml:"And,omitempty" json:"And,omitempty"`
	Tag     Tag      `xml:"Tag,omitempty" json:"Tag,omitempty"`
}

// Validate - validates the filter element
func (f Filter) Validate() error {
	// A Filter must have exactly one of Prefix, Tag, or And specified.
	if !f.And.isEmpty() {
		if f.Prefix != "" {
			return errInvalidFilter
		}
		if !f.Tag.IsEmpty() {
			return errInvalidFilter
		}
	}
	if f.Prefix != "" {
		if !f.Tag.IsEmpty() {
			return errInvalidFilter
		}
	}
	if !f.Tag.IsEmpty() {
		if err := f.Tag.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Tag - a tag for a replication configuration Rule filter.
type Tag struct {
	XMLName xml.Name `json:"-"`
	Key     string   `xml:"Key,omitempty" json:"Key,omitempty"`
	Value   string   `xml:"Value,omitempty" json:"Value,omitempty"`
}

func (tag Tag) String() string {
	return tag.Key + "=" + tag.Value
}

// IsEmpty returns whether this tag is empty or not.
func (tag Tag) IsEmpty() bool {
	return tag.Key == ""
}

// Validate checks this tag.
func (tag Tag) Validate() error {
	if len(tag.Key) == 0 || utf8.RuneCountInString(tag.Key) > 128 {
		return fmt.Errorf("Invalid Tag Key")
	}

	if utf8.RuneCountInString(tag.Value) > 256 {
		return fmt.Errorf("Invalid Tag Value")
	}
	return nil
}

// Destination - destination in ReplicationConfiguration.
type Destination struct {
	XMLName      xml.Name `xml:"Destination" json:"-"`
	Bucket       string   `xml:"Bucket" json:"Bucket"`
	StorageClass string   `xml:"StorageClass,omitempty" json:"StorageClass,omitempty"`
}

// And - a tag to combine a prefix and multiple tags for replication configuration rule.
type And struct {
	XMLName xml.Name `xml:"And,omitempty" json:"-"`
	Prefix  string   `xml:"Prefix,omitempty" json:"Prefix,omitempty"`
	Tags    []Tag    `xml:"Tag,omitempty" json:"Tags,omitempty"`
}

// isEmpty returns true if Tags field is null
func (a And) isEmpty() bool {
	return len(a.Tags) == 0 && a.Prefix == ""
}

// Status represents Enabled/Disabled status
type Status string

// Supported status types
const (
	Enabled  Status = "Enabled"
	Disabled Status = "Disabled"
)

// DeleteMarkerReplication - whether delete markers are replicated - https://docs.aws.amazon.com/AmazonS3/latest/dev/replication-add-config.html
type DeleteMarkerReplication struct {
	Status Status `xml:"Status" json:"Status"` // should be set to "Disabled" by default
}

// IsEmpty returns true if DeleteMarkerReplication is not set
func (d DeleteMarkerReplication) IsEmpty() bool {
	return len(d.Status) == 0
}
