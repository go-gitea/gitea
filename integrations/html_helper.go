// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"

	"golang.org/x/net/html"
)

type HtmlDoc struct {
	doc  *html.Node
	body *html.Node
}

func NewHtmlParser(content []byte) (*HtmlDoc, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	return &HtmlDoc{doc: doc}, nil
}

func (doc *HtmlDoc) GetBody() *html.Node {
	if doc.body == nil {
		var b *html.Node
		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "body" {
				b = n
				return
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(doc.doc)
		if b != nil {
			doc.body = b
		} else {
			doc.body = doc.doc
		}
	}
	return doc.body
}

func (doc *HtmlDoc) GetAttribute(n *html.Node, key string) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

func (doc *HtmlDoc) checkAttr(n *html.Node, attr, val string) bool {
	if n.Type == html.ElementNode {
		s, ok := doc.GetAttribute(n, attr)
		if ok && s == val {
			return true
		}
	}
	return false
}

func (doc *HtmlDoc) traverse(n *html.Node, attr, val string) *html.Node {
	if doc.checkAttr(n, attr, val) {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result := doc.traverse(c, attr, val)
		if result != nil {
			return result
		}
	}

	return nil
}

func (doc *HtmlDoc) GetElementById(id string) *html.Node {
	return doc.traverse(doc.GetBody(), "id", id)
}

func (doc *HtmlDoc) GetInputValueById(id string) string {
	inp := doc.GetElementById(id)
	if inp == nil {
		return ""
	}

	val, _ := doc.GetAttribute(inp, "value")
	return val
}

func (doc *HtmlDoc) GetElementByName(name string) *html.Node {
	return doc.traverse(doc.GetBody(), "name", name)
}

func (doc *HtmlDoc) GetInputValueByName(name string) string {
	inp := doc.GetElementByName(name)
	if inp == nil {
		return ""
	}

	val, _ := doc.GetAttribute(inp, "value")
	return val
}
