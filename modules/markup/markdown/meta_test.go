// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// IssueTemplate is a legacy to keep the unit tests working.
// Copied from structs.IssueTemplate, the original type has been changed a lot to support yaml template.
type IssueTemplate struct {
	Name   string   `json:"name" yaml:"name"`
	Title  string   `json:"title" yaml:"title"`
	About  string   `json:"about" yaml:"about"`
	Labels []string `json:"labels" yaml:"labels"`
	Ref    string   `json:"ref" yaml:"ref"`
}

func (it *IssueTemplate) Valid() bool {
	return strings.TrimSpace(it.Name) != "" && strings.TrimSpace(it.About) != ""
}

func TestExtractMetadata(t *testing.T) {
	for _, mode := range []string{"String", "Bytes"} {
		t.Run(mode, func(t *testing.T) {
			extract := func(format string, args ...any) (string, IssueTemplate, error) {
				var meta IssueTemplate
				if mode == "String" {
					body, err := ExtractMetadata(fmt.Sprintf(format, args...), &meta)
					return body, meta, err
				} else {
					body, err := ExtractMetadataBytes(fmt.Appendf(nil, format, args...), &meta)
					return string(body), meta, err
				}
			}

			t.Run("ValidFrontAndBody", func(t *testing.T) {
				body, meta, err := extract("%s\n%s\n%s\n%s", sepTest, frontTest, sepTest, bodyTest)
				assert.NoError(t, err)
				assert.Equal(t, bodyTest, body)
				assert.Equal(t, metaTest, meta)
				assert.True(t, meta.Valid())
			})

			t.Run("NoFirstSeparator", func(t *testing.T) {
				_, _, err := extract("%s\n%s\n%s", frontTest, sepTest, bodyTest)
				assert.Error(t, err)
			})

			t.Run("NoLastSeparator", func(t *testing.T) {
				_, _, err := extract("%s\n%s\n%s", sepTest, frontTest, bodyTest)
				assert.Error(t, err)
			})

			t.Run("NoBody", func(t *testing.T) {
				body, meta, err := extract("%s\n%s\n%s", sepTest, frontTest, sepTest)
				assert.NoError(t, err)
				assert.Empty(t, body)
				assert.Equal(t, metaTest, meta)
				assert.True(t, meta.Valid())
			})
		})
	}
}

var (
	sepTest   = "-----"
	frontTest = `name: Test
about: "A Test"
title: "Test Title"
labels:
  - bug
  - "test label"`
	bodyTest = "This is the body"
	metaTest = IssueTemplate{
		Name:   "Test",
		About:  "A Test",
		Title:  "Test Title",
		Labels: []string{"bug", "test label"},
	}
)
