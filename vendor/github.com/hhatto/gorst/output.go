/*  Original C version https://github.com/jgm/peg-markdown/
 *	Copyright 2008 John MacFarlane (jgm at berkeley dot edu).
 *
 *  Modifications and translation from C into Go
 *  based on markdown_output.c
 *	Copyright 2010 Michael Teichgräber (mt at wmipf dot de)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License or the MIT
 *  license.  See LICENSE for details.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 */

package rst

// HTML output functions

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
)

type Writer interface {
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	WriteRune(rune) (int, error)
	WriteByte(byte) error
}

type baseWriter struct {
	Writer
	padded int
}

type htmlOut struct {
	baseWriter
	obfuscate bool

	notenum  int
	endNotes []*element /* List of endnotes to print after main content. */
}

func ToHTML(w Writer) Formatter {
	f := new(htmlOut)
	f.baseWriter = baseWriter{w, 2}
	return f
}
func (f *htmlOut) FormatBlock(tree *element) {
	f.elist(tree)
}
func (f *htmlOut) Finish() {
	if len(f.endNotes) != 0 {
		f.sp()
		f.printEndnotes()
	}
	f.WriteByte('\n')
	f.padded = 2
}

// pad - add a number of newlines, the value of the
// argument minus the value of `padded'
// One newline means a line break, similar to troff's .br
// request, two newlines mean a line break plus an
// empty line, similar to troff's .sp request
func (w *baseWriter) pad(n int) {
	for ; n > w.padded; n-- {
		w.WriteByte('\n')
	}
	w.padded = 0
}

func (h *htmlOut) br() *htmlOut {
	h.pad(1)
	return h
}

func (h *htmlOut) sp() *htmlOut {
	h.pad(2)
	return h
}

func (h *htmlOut) skipPadding() *htmlOut {
	h.padded = 2
	return h
}

// print a string
func (w *htmlOut) s(s string) *htmlOut {
	w.WriteString(s)
	return w
}

/* print string, escaping for HTML
 * If obfuscate selected, convert characters to hex or decimal entities at random
 */
func (w *htmlOut) str(s string) *htmlOut {
	var ws string
	var i0 = 0

	o := w.obfuscate
	for i, r := range s {
		switch r {
		case '&':
			ws = "&amp;"
		case '<':
			ws = "&lt;"
		case '>':
			ws = "&gt;"
		case '"':
			ws = "&quot;"
		default:
			if o && r < 128 && r >= 0 {
				if rand.Intn(2) == 0 {
					ws = fmt.Sprintf("&#%d;", r)
				} else {
					ws = fmt.Sprintf("&#%x;", r)
				}
			} else {
				if i0 == -1 {
					i0 = i
				}
				continue
			}
		}
		if i0 != -1 {
			w.WriteString(s[i0:i])
			i0 = -1
		}
		w.WriteString(ws)
	}
	if i0 != -1 {
		w.WriteString(s[i0:])
	}
	return w
}

func (w *htmlOut) children(el *element) *htmlOut {
	return w.elist(el.children)
}
func (w *htmlOut) inline(tag string, el *element) *htmlOut {
	return w.s(tag).children(el).s("</").s(tag[1:])
}
func (w *htmlOut) listBlock(tag string, el *element) *htmlOut {
	return w.sp().s(tag).elist(el.children).br().s("</").s(tag[1:])
}
func (w *htmlOut) listItem(tag string, el *element) *htmlOut {
	return w.br().s(tag).skipPadding().elist(el.children).s("</").s(tag[1:])
}

/* print a list of elements
 */
func (w *htmlOut) elist(list *element) *htmlOut {
	for list != nil {
		w.elem(list)
		list = list.next
	}
	return w
}

// print an element
func (w *htmlOut) elem(elt *element) *htmlOut {
	var s string

	switch elt.key {
	case SPACE:
		s = elt.contents.str
	case LINEBREAK:
		s = "<br/>\n"
	case STR:
		w.str(elt.contents.str)
	case ELLIPSIS:
		s = "&hellip;"
	case EMDASH:
		s = "&mdash;"
	case ENDASH:
		s = "&ndash;"
	case APOSTROPHE:
		s = "&rsquo;"
	case SINGLEQUOTED:
		w.s("&lsquo;").children(elt).s("&rsquo;")
	case DOUBLEQUOTED:
		w.s("&ldquo;").children(elt).s("&rdquo;")
	case CODE:
		w.s("<code>").str(elt.contents.str).s("</code>")
	case HTML:
		s = elt.contents.str
	case LINK:
		o := w.obfuscate
		if strings.Index(elt.contents.link.url, "mailto:") == 0 {
			w.obfuscate = true /* obfuscate mailto: links */
		}
		w.s(`<a href="`).str(elt.contents.link.url).s(`"`)
		if len(elt.contents.link.title) > 0 {
			w.s(` title="`).str(elt.contents.link.title).s(`"`)
		}
		if elt.children != nil && elt.children.key == IMAGE {
			w.s(">").children(elt).s("</a>")
		} else {
			w.s(">").elist(elt.contents.link.label).s("</a>")
		}
		w.obfuscate = o
	case IMAGE:
		w.s(`<img src="`).str(elt.contents.link.url).s(`" alt="`)
		if len(elt.contents.link.title) > 0 {
			w.s(elt.contents.link.title).s(`"`)
		} else {
			w.elist(elt.contents.link.label).s(`"`)
		}
		w.s(" />")
	case EMPH:
		w.inline("<em>", elt)
	case STRONG:
		w.inline("<strong>", elt)
	case STRIKE:
		w.inline("<del>", elt)
	case LIST:
		w.children(elt)
	case RAW:
		/* Shouldn't occur - these are handled by process_raw_blocks() */
		log.Fatalf("RAW")
	case H1TITLE:
		w.sp().inline("<h1 class=\"title\">", elt)
	case H1, H2, H3, H4, H5, H6:
		h := "<h" + string('1'+elt.key-H1) + ">" /* assumes H1 ... H6 are in order */
		w.sp().inline(h, elt)
	case PLAIN:
		w.br().children(elt)
	case PARA:
		w.sp().inline("<p>", elt)
	case HRULE:
		w.sp().s("<hr />")
	case CODEBLOCK:
		w.sp().s(`<pre class="`).str(elt.contents.code.lang).s(`"><code>`)
		w.str(elt.contents.str).s("</code></pre>")
	case HTMLBLOCK:
		w.sp().s(elt.contents.str)
	case VERBATIM:
		w.sp().s("<pre><code>").str(elt.contents.str).s("</code></pre>")
	case TABLE:
		w.listBlock("<table>", elt)
	case TABLESEPARATOR:
		/* None */
	case TABLECELL:
		w.listItem("<td>", elt)
	case CELLSPAN:
		/* None */
	case TABLEROW:
		w.listItem("<tr>", elt)
	case TABLEBODY:
		w.listBlock("<tbody>", elt)
	case TABLEHEAD:
		w.listBlock("<thead>", elt)
	case BULLETLIST:
		w.listBlock("<ul>", elt)
	case ORDEREDLIST:
		w.listBlock("<ol>", elt)
	case DEFINITIONLIST:
		w.listBlock("<dl>", elt)
	case DEFTITLE:
		w.listItem("<dt>", elt)
	case DEFDATA:
		w.listItem("<dd>", elt)
	case LISTITEM:
		w.listItem("<li>", elt)
	case BLOCKQUOTE:
		w.sp().s("<blockquote>\n").skipPadding().children(elt).br().s("</blockquote>")
	case REFERENCE:
		/* Nonprinting */
	case NOTE:
		/* if contents.str == 0, then print note; else ignore, since this
		 * is a note block that has been incorporated into the notes list
		 */
		if elt.contents.str == "" {
			w.endNotes = append(w.endNotes, elt) /* add an endnote to global endnotes list */
			w.notenum++
			nn := w.notenum
			s = fmt.Sprintf(`<a class="noteref" id="fnref%d" href="#fn%d" title="Jump to note %d">[%d]</a>`,
				nn, nn, nn, nn)
		}
	default:
		log.Fatalf("htmlOut.elem encountered unknown element key = %d\n", elt.key)
	}
	if s != "" {
		w.s(s)
	}
	return w
}

func (w *htmlOut) printEndnotes() {
	extraNewline := func() {
		// add an extra newline to maintain
		// compatibility with the C version.
		w.padded--
	}

	counter := 0

	w.s("<hr/>\n<ol id=\"notes\">")
	for _, elt := range w.endNotes {
		counter++
		extraNewline()
		w.br().s(fmt.Sprintf("<li id=\"fn%d\">\n", counter)).skipPadding()
		w.children(elt)
		w.s(fmt.Sprintf(" <a href=\"#fnref%d\" title=\"Jump back to reference\">[back]</a>", counter))
		w.br().s("</li>")
	}
	extraNewline()
	w.br().s("</ol>")
}
