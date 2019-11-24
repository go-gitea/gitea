package org

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
)

type Comment struct{ Content string }

type Keyword struct {
	Key   string
	Value string
}

type NodeWithName struct {
	Name string
	Node Node
}

type NodeWithMeta struct {
	Node Node
	Meta Metadata
}

type Metadata struct {
	Caption        [][]Node
	HTMLAttributes [][]string
}

type Include struct {
	Keyword
	Resolve func() Node
}

var keywordRegexp = regexp.MustCompile(`^(\s*)#\+([^:]+):(\s+(.*)|$)`)
var commentRegexp = regexp.MustCompile(`^(\s*)#(.*)`)

var includeFileRegexp = regexp.MustCompile(`(?i)^"([^"]+)" (src|example|export) (\w+)$`)
var attributeRegexp = regexp.MustCompile(`(?:^|\s+)(:[-\w]+)\s+(.*)$`)

func lexKeywordOrComment(line string) (token, bool) {
	if m := keywordRegexp.FindStringSubmatch(line); m != nil {
		return token{"keyword", len(m[1]), m[2], m}, true
	} else if m := commentRegexp.FindStringSubmatch(line); m != nil {
		return token{"comment", len(m[1]), m[2], m}, true
	}
	return nilToken, false
}

func (d *Document) parseComment(i int, stop stopFn) (int, Node) {
	return 1, Comment{d.tokens[i].content}
}

func (d *Document) parseKeyword(i int, stop stopFn) (int, Node) {
	k := parseKeyword(d.tokens[i])
	switch k.Key {
	case "NAME":
		return d.parseNodeWithName(k, i, stop)
	case "SETUPFILE":
		return d.loadSetupFile(k)
	case "INCLUDE":
		return d.parseInclude(k)
	case "CAPTION", "ATTR_HTML":
		consumed, node := d.parseAffiliated(i, stop)
		if consumed != 0 {
			return consumed, node
		}
		fallthrough
	default:
		if _, ok := d.BufferSettings[k.Key]; ok {
			d.BufferSettings[k.Key] = strings.Join([]string{d.BufferSettings[k.Key], k.Value}, "\n")
		} else {
			d.BufferSettings[k.Key] = k.Value
		}
		return 1, k
	}
}

func (d *Document) parseNodeWithName(k Keyword, i int, stop stopFn) (int, Node) {
	if stop(d, i+1) {
		return 0, nil
	}
	consumed, node := d.parseOne(i+1, stop)
	if consumed == 0 || node == nil {
		return 0, nil
	}
	d.NamedNodes[k.Value] = node
	return consumed + 1, NodeWithName{k.Value, node}
}

func (d *Document) parseAffiliated(i int, stop stopFn) (int, Node) {
	start, meta := i, Metadata{}
	for ; !stop(d, i) && d.tokens[i].kind == "keyword"; i++ {
		switch k := parseKeyword(d.tokens[i]); k.Key {
		case "CAPTION":
			meta.Caption = append(meta.Caption, d.parseInline(k.Value))
		case "ATTR_HTML":
			attributes, rest := []string{}, k.Value
			for {
				if k, m := "", attributeRegexp.FindStringSubmatch(rest); m != nil {
					k, rest = m[1], m[2]
					attributes = append(attributes, k)
					if v, m := "", attributeRegexp.FindStringSubmatchIndex(rest); m != nil {
						v, rest = rest[:m[0]], rest[m[0]:]
						attributes = append(attributes, v)
					} else {
						attributes = append(attributes, strings.TrimSpace(rest))
						break
					}
				} else {
					break
				}
			}
			meta.HTMLAttributes = append(meta.HTMLAttributes, attributes)
		default:
			return 0, nil
		}
	}
	if stop(d, i) {
		return 0, nil
	}
	consumed, node := d.parseOne(i, stop)
	if consumed == 0 || node == nil {
		return 0, nil
	}
	i += consumed
	return i - start, NodeWithMeta{node, meta}
}

func parseKeyword(t token) Keyword {
	k, v := t.matches[2], t.matches[4]
	return Keyword{strings.ToUpper(k), strings.TrimSpace(v)}
}

func (d *Document) parseInclude(k Keyword) (int, Node) {
	resolve := func() Node {
		d.Log.Printf("Bad include %#v", k)
		return k
	}
	if m := includeFileRegexp.FindStringSubmatch(k.Value); m != nil {
		path, kind, lang := m[1], m[2], m[3]
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(d.Path), path)
		}
		resolve = func() Node {
			bs, err := d.ReadFile(path)
			if err != nil {
				d.Log.Printf("Bad include %#v: %s", k, err)
				return k
			}
			return Block{strings.ToUpper(kind), []string{lang}, d.parseRawInline(string(bs))}
		}
	}
	return 1, Include{k, resolve}
}

func (d *Document) loadSetupFile(k Keyword) (int, Node) {
	path := k.Value
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(d.Path), path)
	}
	bs, err := d.ReadFile(path)
	if err != nil {
		d.Log.Printf("Bad setup file: %#v: %s", k, err)
		return 1, k
	}
	setupDocument := d.Configuration.Parse(bytes.NewReader(bs), path)
	if err := setupDocument.Error; err != nil {
		d.Log.Printf("Bad setup file: %#v: %s", k, err)
		return 1, k
	}
	for k, v := range setupDocument.BufferSettings {
		d.BufferSettings[k] = v
	}
	return 1, k
}

func (n Comment) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n Keyword) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n NodeWithMeta) String() string { return orgWriter.WriteNodesAsString(n) }
func (n NodeWithName) String() string { return orgWriter.WriteNodesAsString(n) }
func (n Include) String() string      { return orgWriter.WriteNodesAsString(n) }
