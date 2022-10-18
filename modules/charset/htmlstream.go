// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package charset

import (
	"fmt"
	"io"

	"golang.org/x/net/html"
)

// HTMLStreamer represents a SAX-like interface for HTML
type HTMLStreamer interface {
	Error(err error) error
	Doctype(data string) error
	Comment(data string) error
	StartTag(data string, attrs ...html.Attribute) error
	SelfClosingTag(data string, attrs ...html.Attribute) error
	EndTag(data string) error
	Text(data string) error
}

// PassthroughHTMLStreamer is a passthrough streamer
type PassthroughHTMLStreamer struct {
	next HTMLStreamer
}

func NewPassthroughStreamer(next HTMLStreamer) *PassthroughHTMLStreamer {
	return &PassthroughHTMLStreamer{next: next}
}

var _ (HTMLStreamer) = &PassthroughHTMLStreamer{}

// Error tells the next streamer in line that there is an error
func (p *PassthroughHTMLStreamer) Error(err error) error {
	return p.next.Error(err)
}

// Doctype tells the next streamer what the doctype is
func (p *PassthroughHTMLStreamer) Doctype(data string) error {
	return p.next.Doctype(data)
}

// Comment tells the next streamer there is a comment
func (p *PassthroughHTMLStreamer) Comment(data string) error {
	return p.next.Comment(data)
}

// StartTag tells the next streamer there is a starting tag
func (p *PassthroughHTMLStreamer) StartTag(data string, attrs ...html.Attribute) error {
	return p.next.StartTag(data, attrs...)
}

// SelfClosingTag tells the next streamer there is a self-closing tag
func (p *PassthroughHTMLStreamer) SelfClosingTag(data string, attrs ...html.Attribute) error {
	return p.next.SelfClosingTag(data, attrs...)
}

// EndTag tells the next streamer there is a end tag
func (p *PassthroughHTMLStreamer) EndTag(data string) error {
	return p.next.EndTag(data)
}

// Text tells the next streamer there is a text
func (p *PassthroughHTMLStreamer) Text(data string) error {
	return p.next.Text(data)
}

// HTMLStreamWriter acts as a writing sink
type HTMLStreamerWriter struct {
	io.Writer
	err error
}

// Write implements io.Writer
func (h *HTMLStreamerWriter) Write(data []byte) (int, error) {
	if h.err != nil {
		return 0, h.err
	}
	return h.Writer.Write(data)
}

// Write implements io.StringWriter
func (h *HTMLStreamerWriter) WriteString(data string) (int, error) {
	if h.err != nil {
		return 0, h.err
	}
	return h.Writer.Write([]byte(data))
}

// Error tells the next streamer in line that there is an error
func (h *HTMLStreamerWriter) Error(err error) error {
	if h.err == nil {
		h.err = err
	}
	return h.err
}

// Doctype tells the next streamer what the doctype is
func (h *HTMLStreamerWriter) Doctype(data string) error {
	_, h.err = h.WriteString("<!DOCTYPE " + data + ">")
	return h.err
}

// Comment tells the next streamer there is a comment
func (h *HTMLStreamerWriter) Comment(data string) error {
	_, h.err = h.WriteString("<!--" + data + "-->")
	return h.err
}

// StartTag tells the next streamer there is a starting tag
func (h *HTMLStreamerWriter) StartTag(data string, attrs ...html.Attribute) error {
	return h.startTag(data, attrs, false)
}

// SelfClosingTag tells the next streamer there is a self-closing tag
func (h *HTMLStreamerWriter) SelfClosingTag(data string, attrs ...html.Attribute) error {
	return h.startTag(data, attrs, true)
}

func (h *HTMLStreamerWriter) startTag(data string, attrs []html.Attribute, selfclosing bool) error {
	if _, h.err = h.WriteString("<" + data); h.err != nil {
		return h.err
	}
	for _, attr := range attrs {
		if _, h.err = h.WriteString(" " + attr.Key + "=\"" + html.EscapeString(attr.Val) + "\""); h.err != nil {
			return h.err
		}
	}
	if selfclosing {
		if _, h.err = h.WriteString("/>"); h.err != nil {
			return h.err
		}
	} else {
		if _, h.err = h.WriteString(">"); h.err != nil {
			return h.err
		}
	}
	return h.err
}

// EndTag tells the next streamer there is a end tag
func (h *HTMLStreamerWriter) EndTag(data string) error {
	_, h.err = h.WriteString("</" + data + ">")
	return h.err
}

// Text tells the next streamer there is a text
func (h *HTMLStreamerWriter) Text(data string) error {
	_, h.err = h.WriteString(html.EscapeString(data))
	return h.err
}

// StreamHTML streams an html to a provided streamer
func StreamHTML(source io.Reader, streamer HTMLStreamer) error {
	tokenizer := html.NewTokenizer(source)
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() != io.EOF {
				return tokenizer.Err()
			}
			return nil
		case html.DoctypeToken:
			token := tokenizer.Token()
			if err := streamer.Doctype(token.Data); err != nil {
				return err
			}
		case html.CommentToken:
			token := tokenizer.Token()
			if err := streamer.Comment(token.Data); err != nil {
				return err
			}
		case html.StartTagToken:
			token := tokenizer.Token()
			if err := streamer.StartTag(token.Data, token.Attr...); err != nil {
				return err
			}
		case html.SelfClosingTagToken:
			token := tokenizer.Token()
			if err := streamer.StartTag(token.Data, token.Attr...); err != nil {
				return err
			}
		case html.EndTagToken:
			token := tokenizer.Token()
			if err := streamer.EndTag(token.Data); err != nil {
				return err
			}
		case html.TextToken:
			token := tokenizer.Token()
			if err := streamer.Text(token.Data); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown type of token: %d", tt)
		}
	}
}
