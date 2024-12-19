// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"io"
	"regexp"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/htmlutil"

	"golang.org/x/net/html"
)

var reAttrClass = sync.OnceValue(func() *regexp.Regexp {
	// TODO: it isn't a problem at the moment because our HTML contents are always well constructed
	return regexp.MustCompile(`(<[^>]+)\s+class="([^"]+)"([^>]*>)`)
})

// RenderInternal also works without initialization
// If no initialization (no secureID), it will not protect any attributes and return the original name&value
type RenderInternal struct {
	secureID       string
	secureIDPrefix string
}

func (r *RenderInternal) Init(output io.Writer) io.WriteCloser {
	buf := make([]byte, 12)
	_, err := rand.Read(buf)
	if err != nil {
		panic("unable to generate secure id")
	}
	return r.init(base64.URLEncoding.EncodeToString(buf), output)
}

func (r *RenderInternal) init(secID string, output io.Writer) io.WriteCloser {
	r.secureID = secID
	r.secureIDPrefix = r.secureID + ":"
	return &finalProcessor{renderInternal: r, output: output}
}

func (r *RenderInternal) RecoverProtectedValue(v string) (string, bool) {
	if !strings.HasPrefix(v, r.secureIDPrefix) {
		return "", false
	}
	return v[len(r.secureIDPrefix):], true
}

func (r *RenderInternal) SafeAttr(name string) string {
	if r.secureID == "" {
		return name
	}
	return "data-attr-" + name
}

func (r *RenderInternal) SafeValue(val string) string {
	if r.secureID == "" {
		return val
	}
	return r.secureID + ":" + val
}

func (r *RenderInternal) NodeSafeAttr(attr, val string) html.Attribute {
	return html.Attribute{Key: r.SafeAttr(attr), Val: r.SafeValue(val)}
}

func (r *RenderInternal) ProtectSafeAttrs(content template.HTML) template.HTML {
	if r.secureID == "" {
		return content
	}
	return template.HTML(reAttrClass().ReplaceAllString(string(content), `$1 data-attr-class="`+r.secureIDPrefix+`$2"$3`))
}

func (r *RenderInternal) FormatWithSafeAttrs(w io.Writer, fmt string, a ...any) error {
	_, err := w.Write([]byte(r.ProtectSafeAttrs(htmlutil.HTMLFormat(fmt, a...))))
	return err
}
