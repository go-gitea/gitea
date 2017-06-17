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

type HtmlDoc struct {
	doc *goquery.Document
}

func NewHtmlParser(t testing.TB, content []byte) *HtmlDoc {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	assert.NoError(t, err)
	return &HtmlDoc{doc: doc}
}

func (doc *HtmlDoc) GetInputValueById(id string) string {
	text, _ := doc.doc.Find("#" + id).Attr("value")
	return text
}

func (doc *HtmlDoc) GetInputValueByName(name string) string {
	text, _ := doc.doc.Find("input[name=\"" + name + "\"]").Attr("value")
	return text
}

func (doc *HtmlDoc) GetCSRF() string {
	return doc.GetInputValueByName("_csrf")
}
