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
	t.Run("ValidFrontAndBody", func(t *testing.T) {
		var meta IssueTemplate
		body, err := ExtractMetadata(fmt.Sprintf("%s\n%s\n%s\n%s", sepTest, frontTest, sepTest, bodyTest), &meta)
		assert.NoError(t, err)
		assert.Equal(t, bodyTest, body)
		assert.Equal(t, metaTest, meta)
		assert.True(t, meta.Valid())
	})

	t.Run("NoFirstSeparator", func(t *testing.T) {
		var meta IssueTemplate
		_, err := ExtractMetadata(fmt.Sprintf("%s\n%s\n%s", frontTest, sepTest, bodyTest), &meta)
		assert.Error(t, err)
	})

	t.Run("NoLastSeparator", func(t *testing.T) {
		var meta IssueTemplate
		_, err := ExtractMetadata(fmt.Sprintf("%s\n%s\n%s", sepTest, frontTest, bodyTest), &meta)
		assert.Error(t, err)
	})

	t.Run("NoBody", func(t *testing.T) {
		var meta IssueTemplate
		body, err := ExtractMetadata(fmt.Sprintf("%s\n%s\n%s", sepTest, frontTest, sepTest), &meta)
		assert.NoError(t, err)
		assert.Equal(t, "", body)
		assert.Equal(t, metaTest, meta)
		assert.True(t, meta.Valid())
	})
}

func TestExtractMetadataBytes(t *testing.T) {
	t.Run("ValidFrontAndBody", func(t *testing.T) {
		var meta IssueTemplate
		body, err := ExtractMetadataBytes([]byte(fmt.Sprintf("%s\n%s\n%s\n%s", sepTest, frontTest, sepTest, bodyTest)), &meta)
		assert.NoError(t, err)
		assert.Equal(t, bodyTest, string(body))
		assert.Equal(t, metaTest, meta)
		assert.True(t, meta.Valid())
	})

	t.Run("NoFirstSeparator", func(t *testing.T) {
		var meta IssueTemplate
		_, err := ExtractMetadataBytes([]byte(fmt.Sprintf("%s\n%s\n%s", frontTest, sepTest, bodyTest)), &meta)
		assert.Error(t, err)
	})

	t.Run("NoLastSeparator", func(t *testing.T) {
		var meta IssueTemplate
		_, err := ExtractMetadataBytes([]byte(fmt.Sprintf("%s\n%s\n%s", sepTest, frontTest, bodyTest)), &meta)
		assert.Error(t, err)
	})

	t.Run("NoBody", func(t *testing.T) {
		var meta IssueTemplate
		body, err := ExtractMetadataBytes([]byte(fmt.Sprintf("%s\n%s\n%s", sepTest, frontTest, sepTest)), &meta)
		assert.NoError(t, err)
		assert.Equal(t, "", string(body))
		assert.Equal(t, metaTest, meta)
		assert.True(t, meta.Valid())
	})
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
