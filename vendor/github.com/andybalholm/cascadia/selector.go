package cascadia

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Matcher is the interface for basic selector functionality.
// Match returns whether a selector matches n.
type Matcher interface {
	Match(n *html.Node) bool
}

// Sel is the interface for all the functionality provided by selectors.
type Sel interface {
	Matcher
	Specificity() Specificity

	// Returns a CSS input compiling to this selector.
	String() string

	// Returns a pseudo-element, or an empty string.
	PseudoElement() string
}

// Parse parses a selector. Use `ParseWithPseudoElement`
// if you need support for pseudo-elements.
func Parse(sel string) (Sel, error) {
	p := &parser{s: sel}
	compiled, err := p.parseSelector()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// ParseWithPseudoElement parses a single selector,
// with support for pseudo-element.
func ParseWithPseudoElement(sel string) (Sel, error) {
	p := &parser{s: sel, acceptPseudoElements: true}
	compiled, err := p.parseSelector()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// ParseGroup parses a selector, or a group of selectors separated by commas.
// Use `ParseGroupWithPseudoElements`
// if you need support for pseudo-elements.
func ParseGroup(sel string) (SelectorGroup, error) {
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

// ParseGroupWithPseudoElements parses a selector, or a group of selectors separated by commas.
// It supports pseudo-elements.
func ParseGroupWithPseudoElements(sel string) (SelectorGroup, error) {
	p := &parser{s: sel, acceptPseudoElements: true}
	compiled, err := p.parseSelectorGroup()
	if err != nil {
		return nil, err
	}

	if p.i < len(sel) {
		return nil, fmt.Errorf("parsing %q: %d bytes left over", sel, len(sel)-p.i)
	}

	return compiled, nil
}

// A Selector is a function which tells whether a node matches or not.
//
// This type is maintained for compatibility; I recommend using the newer and
// more idiomatic interfaces Sel and Matcher.
type Selector func(*html.Node) bool

// Compile parses a selector and returns, if successful, a Selector object
// that can be used to match against html.Node objects.
func Compile(sel string) (Selector, error) {
	compiled, err := ParseGroup(sel)
	if err != nil {
		return nil, err
	}

	return Selector(compiled.Match), nil
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

func queryInto(n *html.Node, m Matcher, storage []*html.Node) []*html.Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if m.Match(child) {
			storage = append(storage, child)
		}
		storage = queryInto(child, m, storage)
	}

	return storage
}

// QueryAll returns a slice of all the nodes that match m, from the descendants
// of n.
func QueryAll(n *html.Node, m Matcher) []*html.Node {
	return queryInto(n, m, nil)
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

// Query returns the first node that matches m, from the descendants of n.
// If none matches, it returns nil.
func Query(n *html.Node, m Matcher) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if m.Match(c) {
			return c
		}
		if matched := Query(c, m); matched != nil {
			return matched
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

// Filter returns the nodes that match m.
func Filter(nodes []*html.Node, m Matcher) (result []*html.Node) {
	for _, n := range nodes {
		if m.Match(n) {
			result = append(result, n)
		}
	}
	return result
}

type tagSelector struct {
	tag string
}

// Matches elements with a given tag name.
func (t tagSelector) Match(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == t.tag
}

func (c tagSelector) Specificity() Specificity {
	return Specificity{0, 0, 1}
}

func (c tagSelector) PseudoElement() string {
	return ""
}

type classSelector struct {
	class string
}

// Matches elements by class attribute.
func (t classSelector) Match(n *html.Node) bool {
	return matchAttribute(n, "class", func(s string) bool {
		return matchInclude(t.class, s)
	})
}

func (c classSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c classSelector) PseudoElement() string {
	return ""
}

type idSelector struct {
	id string
}

// Matches elements by id attribute.
func (t idSelector) Match(n *html.Node) bool {
	return matchAttribute(n, "id", func(s string) bool {
		return s == t.id
	})
}

func (c idSelector) Specificity() Specificity {
	return Specificity{1, 0, 0}
}

func (c idSelector) PseudoElement() string {
	return ""
}

type attrSelector struct {
	key, val, operation string
	regexp              *regexp.Regexp
}

// Matches elements by attribute value.
func (t attrSelector) Match(n *html.Node) bool {
	switch t.operation {
	case "":
		return matchAttribute(n, t.key, func(string) bool { return true })
	case "=":
		return matchAttribute(n, t.key, func(s string) bool { return s == t.val })
	case "!=":
		return attributeNotEqualMatch(t.key, t.val, n)
	case "~=":
		// matches elements where the attribute named key is a whitespace-separated list that includes val.
		return matchAttribute(n, t.key, func(s string) bool { return matchInclude(t.val, s) })
	case "|=":
		return attributeDashMatch(t.key, t.val, n)
	case "^=":
		return attributePrefixMatch(t.key, t.val, n)
	case "$=":
		return attributeSuffixMatch(t.key, t.val, n)
	case "*=":
		return attributeSubstringMatch(t.key, t.val, n)
	case "#=":
		return attributeRegexMatch(t.key, t.regexp, n)
	default:
		panic(fmt.Sprintf("unsuported operation : %s", t.operation))
	}
}

// matches elements where the attribute named key satisifes the function f.
func matchAttribute(n *html.Node, key string, f func(string) bool) bool {
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

// attributeNotEqualMatch matches elements where
// the attribute named key does not have the value val.
func attributeNotEqualMatch(key, val string, n *html.Node) bool {
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

// returns true if s is a whitespace-separated list that includes val.
func matchInclude(val, s string) bool {
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
}

//  matches elements where the attribute named key equals val or starts with val plus a hyphen.
func attributeDashMatch(key, val string, n *html.Node) bool {
	return matchAttribute(n, key,
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

// attributePrefixMatch returns a Selector that matches elements where
// the attribute named key starts with val.
func attributePrefixMatch(key, val string, n *html.Node) bool {
	return matchAttribute(n, key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			return strings.HasPrefix(s, val)
		})
}

// attributeSuffixMatch matches elements where
// the attribute named key ends with val.
func attributeSuffixMatch(key, val string, n *html.Node) bool {
	return matchAttribute(n, key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			return strings.HasSuffix(s, val)
		})
}

// attributeSubstringMatch matches nodes where
// the attribute named key contains val.
func attributeSubstringMatch(key, val string, n *html.Node) bool {
	return matchAttribute(n, key,
		func(s string) bool {
			if strings.TrimSpace(s) == "" {
				return false
			}
			return strings.Contains(s, val)
		})
}

// attributeRegexMatch  matches nodes where
// the attribute named key matches the regular expression rx
func attributeRegexMatch(key string, rx *regexp.Regexp, n *html.Node) bool {
	return matchAttribute(n, key,
		func(s string) bool {
			return rx.MatchString(s)
		})
}

func (c attrSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c attrSelector) PseudoElement() string {
	return ""
}

// ---------------- Pseudo class selectors ----------------
// we use severals concrete types of pseudo-class selectors

type relativePseudoClassSelector struct {
	name  string // one of "not", "has", "haschild"
	match SelectorGroup
}

func (s relativePseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch s.name {
	case "not":
		// matches elements that do not match a.
		return !s.match.Match(n)
	case "has":
		//  matches elements with any descendant that matches a.
		return hasDescendantMatch(n, s.match)
	case "haschild":
		// matches elements with a child that matches a.
		return hasChildMatch(n, s.match)
	default:
		panic(fmt.Sprintf("unsupported relative pseudo class selector : %s", s.name))
	}
}

// hasChildMatch returns whether n has any child that matches a.
func hasChildMatch(n *html.Node, a Matcher) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if a.Match(c) {
			return true
		}
	}
	return false
}

// hasDescendantMatch performs a depth-first search of n's descendants,
// testing whether any of them match a. It returns true as soon as a match is
// found, or false if no match is found.
func hasDescendantMatch(n *html.Node, a Matcher) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if a.Match(c) || (c.Type == html.ElementNode && hasDescendantMatch(c, a)) {
			return true
		}
	}
	return false
}

// Specificity returns the specificity of the most specific selectors
// in the pseudo-class arguments.
// See https://www.w3.org/TR/selectors/#specificity-rules
func (s relativePseudoClassSelector) Specificity() Specificity {
	var max Specificity
	for _, sel := range s.match {
		newSpe := sel.Specificity()
		if max.Less(newSpe) {
			max = newSpe
		}
	}
	return max
}

func (c relativePseudoClassSelector) PseudoElement() string {
	return ""
}

type containsPseudoClassSelector struct {
	own   bool
	value string
}

func (s containsPseudoClassSelector) Match(n *html.Node) bool {
	var text string
	if s.own {
		// matches nodes that directly contain the given text
		text = strings.ToLower(nodeOwnText(n))
	} else {
		// matches nodes that contain the given text.
		text = strings.ToLower(nodeText(n))
	}
	return strings.Contains(text, s.value)
}

func (s containsPseudoClassSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c containsPseudoClassSelector) PseudoElement() string {
	return ""
}

type regexpPseudoClassSelector struct {
	own    bool
	regexp *regexp.Regexp
}

func (s regexpPseudoClassSelector) Match(n *html.Node) bool {
	var text string
	if s.own {
		// matches nodes whose text directly matches the specified regular expression
		text = nodeOwnText(n)
	} else {
		// matches nodes whose text matches the specified regular expression
		text = nodeText(n)
	}
	return s.regexp.MatchString(text)
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

func (s regexpPseudoClassSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c regexpPseudoClassSelector) PseudoElement() string {
	return ""
}

type nthPseudoClassSelector struct {
	a, b         int
	last, ofType bool
}

func (s nthPseudoClassSelector) Match(n *html.Node) bool {
	if s.a == 0 {
		if s.last {
			return simpleNthLastChildMatch(s.b, s.ofType, n)
		} else {
			return simpleNthChildMatch(s.b, s.ofType, n)
		}
	}
	return nthChildMatch(s.a, s.b, s.last, s.ofType, n)
}

// nthChildMatch implements :nth-child(an+b).
// If last is true, implements :nth-last-child instead.
// If ofType is true, implements :nth-of-type instead.
func nthChildMatch(a, b int, last, ofType bool, n *html.Node) bool {
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

// simpleNthChildMatch implements :nth-child(b).
// If ofType is true, implements :nth-of-type instead.
func simpleNthChildMatch(b int, ofType bool, n *html.Node) bool {
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

// simpleNthLastChildMatch implements :nth-last-child(b).
// If ofType is true, implements :nth-last-of-type instead.
func simpleNthLastChildMatch(b int, ofType bool, n *html.Node) bool {
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

// Specificity for nth-child pseudo-class.
// Does not support a list of selectors
func (s nthPseudoClassSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c nthPseudoClassSelector) PseudoElement() string {
	return ""
}

type onlyChildPseudoClassSelector struct {
	ofType bool
}

// Match implements :only-child.
// If `ofType` is true, it implements :only-of-type instead.
func (s onlyChildPseudoClassSelector) Match(n *html.Node) bool {
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
		if (c.Type != html.ElementNode) || (s.ofType && c.Data != n.Data) {
			continue
		}
		count++
		if count > 1 {
			return false
		}
	}

	return count == 1
}

func (s onlyChildPseudoClassSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c onlyChildPseudoClassSelector) PseudoElement() string {
	return ""
}

type inputPseudoClassSelector struct{}

// Matches input, select, textarea and button elements.
func (s inputPseudoClassSelector) Match(n *html.Node) bool {
	return n.Type == html.ElementNode && (n.Data == "input" || n.Data == "select" || n.Data == "textarea" || n.Data == "button")
}

func (s inputPseudoClassSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c inputPseudoClassSelector) PseudoElement() string {
	return ""
}

type emptyElementPseudoClassSelector struct{}

// Matches empty elements.
func (s emptyElementPseudoClassSelector) Match(n *html.Node) bool {
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

func (s emptyElementPseudoClassSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c emptyElementPseudoClassSelector) PseudoElement() string {
	return ""
}

type rootPseudoClassSelector struct{}

// Match implements :root
func (s rootPseudoClassSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if n.Parent == nil {
		return false
	}
	return n.Parent.Type == html.DocumentNode
}

func (s rootPseudoClassSelector) Specificity() Specificity {
	return Specificity{0, 1, 0}
}

func (c rootPseudoClassSelector) PseudoElement() string {
	return ""
}

type compoundSelector struct {
	selectors     []Sel
	pseudoElement string
}

// Matches elements if each sub-selectors matches.
func (t compoundSelector) Match(n *html.Node) bool {
	if len(t.selectors) == 0 {
		return n.Type == html.ElementNode
	}

	for _, sel := range t.selectors {
		if !sel.Match(n) {
			return false
		}
	}
	return true
}

func (s compoundSelector) Specificity() Specificity {
	var out Specificity
	for _, sel := range s.selectors {
		out = out.Add(sel.Specificity())
	}
	if s.pseudoElement != "" {
		// https://drafts.csswg.org/selectors-3/#specificity
		out = out.Add(Specificity{0, 0, 1})
	}
	return out
}

func (c compoundSelector) PseudoElement() string {
	return c.pseudoElement
}

type combinedSelector struct {
	first      Sel
	combinator byte
	second     Sel
}

func (t combinedSelector) Match(n *html.Node) bool {
	if t.first == nil {
		return false // maybe we should panic
	}
	switch t.combinator {
	case 0:
		return t.first.Match(n)
	case ' ':
		return descendantMatch(t.first, t.second, n)
	case '>':
		return childMatch(t.first, t.second, n)
	case '+':
		return siblingMatch(t.first, t.second, true, n)
	case '~':
		return siblingMatch(t.first, t.second, false, n)
	default:
		panic("unknown combinator")
	}
}

// matches an element if it matches d and has an ancestor that matches a.
func descendantMatch(a, d Matcher, n *html.Node) bool {
	if !d.Match(n) {
		return false
	}

	for p := n.Parent; p != nil; p = p.Parent {
		if a.Match(p) {
			return true
		}
	}

	return false
}

// matches an element if it matches d and its parent matches a.
func childMatch(a, d Matcher, n *html.Node) bool {
	return d.Match(n) && n.Parent != nil && a.Match(n.Parent)
}

// matches an element if it matches s2 and is preceded by an element that matches s1.
// If adjacent is true, the sibling must be immediately before the element.
func siblingMatch(s1, s2 Matcher, adjacent bool, n *html.Node) bool {
	if !s2.Match(n) {
		return false
	}

	if adjacent {
		for n = n.PrevSibling; n != nil; n = n.PrevSibling {
			if n.Type == html.TextNode || n.Type == html.CommentNode {
				continue
			}
			return s1.Match(n)
		}
		return false
	}

	// Walk backwards looking for element that matches s1
	for c := n.PrevSibling; c != nil; c = c.PrevSibling {
		if s1.Match(c) {
			return true
		}
	}

	return false
}

func (s combinedSelector) Specificity() Specificity {
	spec := s.first.Specificity()
	if s.second != nil {
		spec = spec.Add(s.second.Specificity())
	}
	return spec
}

// on combinedSelector, a pseudo-element only makes sens on the last
// selector, although others increase specificity.
func (c combinedSelector) PseudoElement() string {
	if c.second == nil {
		return ""
	}
	return c.second.PseudoElement()
}

// A SelectorGroup is a list of selectors, which matches if any of the
// individual selectors matches.
type SelectorGroup []Sel

// Match returns true if the node matches one of the single selectors.
func (s SelectorGroup) Match(n *html.Node) bool {
	for _, sel := range s {
		if sel.Match(n) {
			return true
		}
	}
	return false
}
