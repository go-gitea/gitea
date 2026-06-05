// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

// HTMLDoc struct
type HTMLDoc struct {
	doc *goquery.Document
}

// NewHTMLParser parse html file
func NewHTMLParser(t testing.TB, body io.Reader) *HTMLDoc {
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

// AssertHTMLElement check if the element by selector exists or does not exist depending on checkExists
func AssertHTMLElement[T int | bool](t testing.TB, doc *HTMLDoc, selector string, checkExists T) {
	sel := doc.doc.Find(selector)
	switch v := any(checkExists).(type) {
	case bool:
		assert.Equal(t, v, sel.Length() > 0)
	case int:
		assert.Equal(t, v, sel.Length())
	}
}

func assertHTMLEq(t testing.TB, expected, actual string) {
	t.Helper()
	if expected == actual {
		return
	}
	exp, err := html.Parse(strings.NewReader(expected))
	if !assert.NoError(t, err) {
		return
	}
	act, err := html.Parse(strings.NewReader(actual))
	if !assert.NoError(t, err) {
		return
	}
	var normalize func(n *html.Node)
	normalize = func(n *html.Node) {
		slices.SortFunc(n.Attr, func(a, b html.Attribute) int {
			if cmp := strings.Compare(a.Namespace, b.Namespace); cmp != 0 {
				return cmp
			}
			if cmp := strings.Compare(a.Key, b.Key); cmp != 0 {
				return cmp
			}
			return strings.Compare(a.Val, b.Val)
		})
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			normalize(c)
		}
	}
	normalize(exp)
	normalize(act)
	var expNormalized, actNormalized strings.Builder
	assert.NoError(t, html.Render(&expNormalized, exp))
	assert.NoError(t, html.Render(&actNormalized, act))
	assert.Equal(t, expNormalized.String(), actNormalized.String())
}
