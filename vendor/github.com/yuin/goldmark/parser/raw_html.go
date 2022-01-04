package parser

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type rawHTMLParser struct {
}

var defaultRawHTMLParser = &rawHTMLParser{}

// NewRawHTMLParser return a new InlineParser that can parse
// inline htmls
func NewRawHTMLParser() InlineParser {
	return defaultRawHTMLParser
}

func (s *rawHTMLParser) Trigger() []byte {
	return []byte{'<'}
}

func (s *rawHTMLParser) Parse(parent ast.Node, block text.Reader, pc Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) > 1 && util.IsAlphaNumeric(line[1]) {
		return s.parseMultiLineRegexp(openTagRegexp, block, pc)
	}
	if len(line) > 2 && line[1] == '/' && util.IsAlphaNumeric(line[2]) {
		return s.parseMultiLineRegexp(closeTagRegexp, block, pc)
	}
	if bytes.HasPrefix(line, []byte("<!--")) {
		return s.parseMultiLineRegexp(commentRegexp, block, pc)
	}
	if bytes.HasPrefix(line, []byte("<?")) {
		return s.parseSingleLineRegexp(processingInstructionRegexp, block, pc)
	}
	if len(line) > 2 && line[1] == '!' && line[2] >= 'A' && line[2] <= 'Z' {
		return s.parseSingleLineRegexp(declRegexp, block, pc)
	}
	if bytes.HasPrefix(line, []byte("<![CDATA[")) {
		return s.parseMultiLineRegexp(cdataRegexp, block, pc)
	}
	return nil
}

var tagnamePattern = `([A-Za-z][A-Za-z0-9-]*)`
var attributePattern = `(?:\s+[a-zA-Z_:][a-zA-Z0-9:._-]*(?:\s*=\s*(?:[^\"'=<>` + "`" + `\x00-\x20]+|'[^']*'|"[^"]*"))?)`
var openTagRegexp = regexp.MustCompile("^<" + tagnamePattern + attributePattern + `*\s*/?>`)
var closeTagRegexp = regexp.MustCompile("^</" + tagnamePattern + `\s*>`)
var commentRegexp = regexp.MustCompile(`^<!---->|<!--(?:-?[^>-])(?:-?[^-])*-->`)
var processingInstructionRegexp = regexp.MustCompile(`^(?:<\?).*?(?:\?>)`)
var declRegexp = regexp.MustCompile(`^<![A-Z]+\s+[^>]*>`)
var cdataRegexp = regexp.MustCompile(`<!\[CDATA\[[\s\S]*?\]\]>`)

func (s *rawHTMLParser) parseSingleLineRegexp(reg *regexp.Regexp, block text.Reader, pc Context) ast.Node {
	line, segment := block.PeekLine()
	match := reg.FindSubmatchIndex(line)
	if match == nil {
		return nil
	}
	node := ast.NewRawHTML()
	node.Segments.Append(segment.WithStop(segment.Start + match[1]))
	block.Advance(match[1])
	return node
}

func (s *rawHTMLParser) parseMultiLineRegexp(reg *regexp.Regexp, block text.Reader, pc Context) ast.Node {
	sline, ssegment := block.Position()
	if block.Match(reg) {
		node := ast.NewRawHTML()
		eline, esegment := block.Position()
		block.SetPosition(sline, ssegment)
		for {
			line, segment := block.PeekLine()
			if line == nil {
				break
			}
			l, _ := block.Position()
			start := segment.Start
			if l == sline {
				start = ssegment.Start
			}
			end := segment.Stop
			if l == eline {
				end = esegment.Start
			}

			node.Segments.Append(text.NewSegment(start, end))
			if l == eline {
				block.Advance(end - start)
				break
			} else {
				block.AdvanceLine()
			}
		}
		return node
	}
	return nil
}
