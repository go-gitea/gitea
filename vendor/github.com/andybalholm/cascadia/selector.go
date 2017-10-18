package cascadia

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// the Selector type, and functions for creating them

// A Selector is a function which tells whether a node matches or not.
type Selector func(*html.Node) bool

// hasChildMatch returns whether n has any child that matches a.
func hasChildMatch(n *html.Node, a Selector) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if a(c) {
			return true
		}
	}
	return false
}

// hasDescendantMatch performs a depth-first search of n's descendants,
// testing whether any of them match a. It returns true as soon as a match is
// found, or false if no match is found.
func hasDescendantMatch(n *html.Node, a Selector) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if a(c) || (c.Type == html.ElementNode && hasDescendantMatch(c, a)) {
			return true
		}
	}
	return false
}

// Compile parses a selector and returns, if successful, a Selector object
// that can be used to match against html.Node objects.
func Compile(sel string) (Selector, error) {
	p := &parser{s: sel}
	compiled, err := p.parseSelectorGroup()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// MustCompile is like Compile, but panics instead of returning an error.
func MustCompile(sel string) Selector {
	compiled, err := Compile(sel)
	if err != nil {
		panic(err)
	}
	return compiled
}

// MatchAll returns a slice of the nodes that match the selector,
// from n and its children.
func (s Selector) MatchAll(n *html.Node) []*html.Node {
	return s.matchAllInto(n, nil)
}

func (s Selector) matchAllInto(n *html.Node, storage []*html.Node) []*html.Node {
	if s(n) {
		storage = append(storage, n)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		storage = s.matchAllInto(child, storage)
	}

	return storage
}

// Match returns true if the node matches the selector.
func (s Selector) Match(n *html.Node) bool {
	return s(n)
}

// MatchFirst returns the first node that matches s, from n and its children.
func (s Selector) MatchFirst(n *html.Node) *html.Node {
	if s.Match(n) {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		m := s.MatchFirst(c)
		if m != nil {
			return m
		}
	}
	return nil
}

// Filter returns the nodes in nodes that match the selector.
func (s Selector) Filter(nodes []*html.Node) (result []*html.Node) {
	for _, n := range nodes {
		if s(n) {
			result = append(result, n)
		}
	}
	return result
}

// typeSelector returns a Selector that matches elements with a given tag name.
func typeSelector(tag string) Selector {
	tag = toLowerASCII(tag)
	return func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == tag
	}
}

// toLowerASCII returns s with all ASCII capital letters lowercased.
func toLowerASCII(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if c := s[i]; 'A' <= c && c <= 'Z' {
			if b == nil {
				b = make([]byte, len(s))
				copy(b, s)
			}
			b[i] = s[i] + ('a' - 'A')
		}
	}

	if b == nil {
		return s
	}

	return string(b)
}

// attributeSelector returns a Selector that matches elements
// where the attribute named key satisifes the function f.
func attributeSelector(key string, f func(string) bool) Selector {
	key = toLowerASCII(key)
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		for _, a := range n.Attr {
			if a.Key == key && f(a.Val) {
				return true
			}
		}
		return false
	}
}

// attributeExistsSelector returns a Selector that matches elements that have
// an attribute named key.
func attributeExistsSelector(key string) Selector {
	return attributeSelector(key, func(string) bool { return true })
}

// attributeEqualsSelector returns a Selector that matches elements where
// the attribute named key has the value val.
func attributeEqualsSelector(key, val string) Selector {
	return attributeSelector(key,
		func(s string) bool {
			return s == val
		})
}

// attributeNotEqualSelector returns a Selector that matches elements where
// the attribute named key does not have the value val.
func attributeNotEqualSelector(key, val string) Selector {
	key = toLowerASCII(key)
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		for _, a := range n.Attr {
			if a.Key == key && a.Val == val {
				return false
			}
		}
		return true
	}
}

// attributeIncludesSelector returns a Selector that matches elements where
// the attribute named key is a whitespace-separated list that includes val.
func attributeIncludesSelector(key, val string) Selector {
	return attributeSelector(key,
		func(s string) bool {
			for s != "" {
				i := strings.IndexAny(s, " \t\r\n\f")
				if i == -1 {
					return s == val
				}
				if s[:i] == val {
					return true
				}
				s = s[i+1:]
			}
			return false
		})
}

// attributeDashmatchSelector returns a Selector that matches elements where
// the attribute named key equals val or starts with val plus a hyphen.
func attributeDashmatchSelector(key, val string) Selector {
	return attributeSelector(key,
		func(s string) bool {
			if s == val {
				return true
			}
			if len(s) <= len(val) {
				return false
			}
			if s[:len(val)] == val && s[len(val)] == '-' {
				return true
			}
			return false
		})
}

// attributePrefixSelector returns a Selector that matches elements where
// the attribute named key starts with val.
func attributePrefixSelector(key, val string) Selector {
	return attributeSelector(key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			return strings.HasPrefix(s, val)
		})
}

// attributeSuffixSelector returns a Selector that matches elements where
// the attribute named key ends with val.
func attributeSuffixSelector(key, val string) Selector {
	return attributeSelector(key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			return strings.HasSuffix(s, val)
		})
}

// attributeSubstringSelector returns a Selector that matches nodes where
// the attribute named key contains val.
func attributeSubstringSelector(key, val string) Selector {
	return attributeSelector(key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			return strings.Contains(s, val)
		})
}

// attributeRegexSelector returns a Selector that matches nodes where
// the attribute named key matches the regular expression rx
func attributeRegexSelector(key string, rx *regexp.Regexp) Selector {
	return attributeSelector(key,
		func(s string) bool {
			return rx.MatchString(s)
		})
}

// intersectionSelector returns a selector that matches nodes that match
// both a and b.
func intersectionSelector(a, b Selector) Selector {
	return func(n *html.Node) bool {
		return a(n) && b(n)
	}
}

// unionSelector returns a selector that matches elements that match
// either a or b.
func unionSelector(a, b Selector) Selector {
	return func(n *html.Node) bool {
		return a(n) || b(n)
	}
}

// negatedSelector returns a selector that matches elements that do not match a.
func negatedSelector(a Selector) Selector {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		return !a(n)
	}
}

// writeNodeText writes the text contained in n and its descendants to b.
func writeNodeText(n *html.Node, b *bytes.Buffer) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(n.Data)
	case html.ElementNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeNodeText(c, b)
		}
	}
}

// nodeText returns the text contained in n and its descendants.
func nodeText(n *html.Node) string {
	var b bytes.Buffer
	writeNodeText(n, &b)
	return b.String()
}

// nodeOwnText returns the contents of the text nodes that are direct
// children of n.
func nodeOwnText(n *html.Node) string {
	var b bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

// textSubstrSelector returns a selector that matches nodes that
// contain the given text.
func textSubstrSelector(val string) Selector {
	return func(n *html.Node) bool {
		text := strings.ToLower(nodeText(n))
		return strings.Contains(text, val)
	}
}

// ownTextSubstrSelector returns a selector that matches nodes that
// directly contain the given text
func ownTextSubstrSelector(val string) Selector {
	return func(n *html.Node) bool {
		text := strings.ToLower(nodeOwnText(n))
		return strings.Contains(text, val)
	}
}

// textRegexSelector returns a selector that matches nodes whose text matches
// the specified regular expression
func textRegexSelector(rx *regexp.Regexp) Selector {
	return func(n *html.Node) bool {
		return rx.MatchString(nodeText(n))
	}
}

// ownTextRegexSelector returns a selector that matches nodes whose text
// directly matches the specified regular expression
func ownTextRegexSelector(rx *regexp.Regexp) Selector {
	return func(n *html.Node) bool {
		return rx.MatchString(nodeOwnText(n))
	}
}

// hasChildSelector returns a selector that matches elements
// with a child that matches a.
func hasChildSelector(a Selector) Selector {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		return hasChildMatch(n, a)
	}
}

// hasDescendantSelector returns a selector that matches elements
// with any descendant that matches a.
func hasDescendantSelector(a Selector) Selector {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		return hasDescendantMatch(n, a)
	}
}

// nthChildSelector returns a selector that implements :nth-child(an+b).
// If last is true, implements :nth-last-child instead.
// If ofType is true, implements :nth-of-type instead.
func nthChildSelector(a, b int, last, ofType bool) Selector {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}

		parent := n.Parent
		if parent == nil {
			return false
		}

		if parent.Type == html.DocumentNode {
			return false
		}

		i := -1
		count := 0
		for c := parent.FirstChild; c != nil; c = c.NextSibling {
			if (c.Type != html.ElementNode) || (ofType && c.Data != n.Data) {
				continue
			}
			count++
			if c == n {
				i = count
				if !last {
					break
				}
			}
		}

		if i == -1 {
			// This shouldn't happen, since n should always be one of its parent's children.
			return false
		}

		if last {
			i = count - i + 1
		}

		i -= b
		if a == 0 {
			return i == 0
		}

		return i%a == 0 && i/a >= 0
	}
}

// simpleNthChildSelector returns a selector that implements :nth-child(b).
// If ofType is true, implements :nth-of-type instead.
func simpleNthChildSelector(b int, ofType bool) Selector {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}

		parent := n.Parent
		if parent == nil {
			return false
		}

		if parent.Type == html.DocumentNode {
			return false
		}

		count := 0
		for c := parent.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode || (ofType && c.Data != n.Data) {
				continue
			}
			count++
			if c == n {
				return count == b
			}
			if count >= b {
				return false
			}
		}
		return false
	}
}

// simpleNthLastChildSelector returns a selector that implements
// :nth-last-child(b). If ofType is true, implements :nth-last-of-type
// instead.
func simpleNthLastChildSelector(b int, ofType bool) Selector {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}

		parent := n.Parent
		if parent == nil {
			return false
		}

		if parent.Type == html.DocumentNode {
			return false
		}

		count := 0
		for c := parent.LastChild; c != nil; c = c.PrevSibling {
			if c.Type != html.ElementNode || (ofType && c.Data != n.Data) {
				continue
			}
			count++
			if c == n {
				return count == b
			}
			if count >= b {
				return false
			}
		}
		return false
	}
}

// onlyChildSelector returns a selector that implements :only-child.
// If ofType is true, it implements :only-of-type instead.
func onlyChildSelector(ofType bool) Selector {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}

		parent := n.Parent
		if parent == nil {
			return false
		}

		if parent.Type == html.DocumentNode {
			return false
		}

		count := 0
		for c := parent.FirstChild; c != nil; c = c.NextSibling {
			if (c.Type != html.ElementNode) || (ofType && c.Data != n.Data) {
				continue
			}
			count++
			if count > 1 {
				return false
			}
		}

		return count == 1
	}
}

// inputSelector is a Selector that matches input, select, textarea and button elements.
func inputSelector(n *html.Node) bool {
	return n.Type == html.ElementNode && (n.Data == "input" || n.Data == "select" || n.Data == "textarea" || n.Data == "button")
}

// emptyElementSelector is a Selector that matches empty elements.
func emptyElementSelector(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.ElementNode, html.TextNode:
			return false
		}
	}

	return true
}

// descendantSelector returns a Selector that matches an element if
// it matches d and has an ancestor that matches a.
func descendantSelector(a, d Selector) Selector {
	return func(n *html.Node) bool {
		if !d(n) {
			return false
		}

		for p := n.Parent; p != nil; p = p.Parent {
			if a(p) {
				return true
			}
		}

		return false
	}
}

// childSelector returns a Selector that matches an element if
// it matches d and its parent matches a.
func childSelector(a, d Selector) Selector {
	return func(n *html.Node) bool {
		return d(n) && n.Parent != nil && a(n.Parent)
	}
}

// siblingSelector returns a Selector that matches an element
// if it matches s2 and in is preceded by an element that matches s1.
// If adjacent is true, the sibling must be immediately before the element.
func siblingSelector(s1, s2 Selector, adjacent bool) Selector {
	return func(n *html.Node) bool {
		if !s2(n) {
			return false
		}

		if adjacent {
			for n = n.PrevSibling; n != nil; n = n.PrevSibling {
				if n.Type == html.TextNode || n.Type == html.CommentNode {
					continue
				}
				return s1(n)
			}
			return false
		}

		// Walk backwards looking for element that matches s1
		for c := n.PrevSibling; c != nil; c = c.PrevSibling {
			if s1(c) {
				return true
			}
		}

		return false
	}
}

// rootSelector implements :root
func rootSelector(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if n.Parent == nil {
		return false
	}
	return n.Parent.Type == html.DocumentNode
}
