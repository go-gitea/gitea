// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

// HTMLDoc struct
type HTMLDoc struct {
	doc *goquery.Document
}

// NewHTMLParser parse html file
func NewHTMLParser(t testing.TB, body *bytes.Buffer) *HTMLDoc {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(body)
	assert.NoError(t, err)
	return &HTMLDoc{doc: doc}
}

// GetInputValueByName for get input value by name
func (doc *HTMLDoc) GetInputValueByName(name string) string {
	text, _ := doc.doc.Find(`input[name="` + name + `"]`).Attr("value")
	return text
}

// Find gets the descendants of each element in the current set of
// matched elements, filtered by a selector. It returns a new Selection
// object containing these matched elements.
func (doc *HTMLDoc) Find(selector string) *goquery.Selection {
	return doc.doc.Find(selector)
}

// GetCSRF for getting CSRF token value from input
func (doc *HTMLDoc) GetCSRF() string {
	return doc.GetInputValueByName("_csrf")
}

// AssertHTMLElement check if element by selector exists or does not exist depending on checkExists
func AssertHTMLElement[T int | bool](t testing.TB, doc *HTMLDoc, selector string, checkExists T) {
	sel := doc.doc.Find(selector)
	switch v := any(checkExists).(type) {
	case bool:
		assert.Equal(t, v, sel.Length() > 0)
	case int:
		assert.Equal(t, v, sel.Length())
	}
}
