// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

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

// GetInputValueByID for get input value by id
func (doc *HTMLDoc) GetInputValueByID(id string) string {
	text, _ := doc.doc.Find("#" + id).Attr("value")
	return text
}

// GetInputValueByName for get input value by name
func (doc *HTMLDoc) GetInputValueByName(name string) string {
	text, _ := doc.doc.Find("input[name=\"" + name + "\"]").Attr("value")
	return text
}

// GetCSRF for get CSRC token value from input
func (doc *HTMLDoc) GetCSRF() string {
	return doc.GetInputValueByName("_csrf")
}

// AssertElement check if element by selector exists or does not exist depending on checkExists
func (doc *HTMLDoc) AssertElement(t testing.TB, selector string, checkExists bool) {
	sel := doc.doc.Find(selector)
	if checkExists {
		assert.Equal(t, 1, sel.Length())
	} else {
		assert.Equal(t, 0, sel.Length())
	}
}
