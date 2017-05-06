// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"

	"github.com/PuerkitoBio/goquery"
)

type HtmlDoc struct {
	doc *goquery.Document
}

func NewHtmlParser(content []byte) (*HtmlDoc, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	return &HtmlDoc{doc: doc}, nil
}

func (doc *HtmlDoc) GetInputValueById(id string) string {
	text, _ := doc.doc.Find("#" + id).Attr("value")
	return text
}

func (doc *HtmlDoc) GetInputValueByName(name string) string {
	text, _ := doc.doc.Find("input[name=\"" + name + "\"]").Attr("value")
	return text
}
