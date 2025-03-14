// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"io"
	"net/url"
	"regexp"

	"code.gitea.io/gitea/modules/setting"

	"github.com/microcosm-cc/bluemonday"
)

func (st *Sanitizer) createDefaultPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()

	// NOTICE: DO NOT add special "class" regexp rules here anymore, use RenderInternal.SafeAttr instead

	// General safe SVG attributes
	policy.AllowAttrs("viewBox", "width", "height", "aria-hidden", "data-attr-class").OnElements("svg")
	policy.AllowAttrs("fill-rule", "d").OnElements("path")

	// Checkboxes
	policy.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	policy.AllowAttrs("checked", "disabled", "data-source-position").OnElements("input")

	// Chroma always uses 1-2 letters for style names, we could tolerate it at the moment
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^\w{0,2}$`)).OnElements("span")

	// Custom URL-Schemes
	if len(setting.Markdown.CustomURLSchemes) > 0 {
		policy.AllowURLSchemes(setting.Markdown.CustomURLSchemes...)
	} else {
		policy.AllowURLSchemesMatching(st.allowAllRegex)

		// Even if every scheme is allowed, these three are blocked for security reasons
		disallowScheme := func(*url.URL) bool {
			return false
		}
		policy.AllowURLSchemeWithCustomPolicy("javascript", disallowScheme)
		policy.AllowURLSchemeWithCustomPolicy("vbscript", disallowScheme)
		policy.AllowURLSchemeWithCustomPolicy("data", disallowScheme)
	}

	// Allow classes for org mode list item status.
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^(unchecked|checked|indeterminate)$`)).OnElements("li")

	// Allow 'color' and 'background-color' properties for the style attribute on text elements.
	policy.AllowStyles("color", "background-color").OnElements("div", "span", "p", "tr", "th", "td")

	policy.AllowAttrs("src", "autoplay", "controls").OnElements("video")

	// Allow generally safe attributes (reference: https://github.com/jch/html-pipeline)
	generalSafeAttrs := []string{
		"abbr", "accept", "accept-charset",
		"accesskey", "action", "align", "alt",
		"aria-describedby", "aria-hidden", "aria-label", "aria-labelledby",
		"axis", "border", "cellpadding", "cellspacing", "char",
		"charoff", "charset", "checked",
		"clear", "cols", "colspan", "color",
		"compact", "coords", "datetime", "dir",
		"disabled", "enctype", "for", "frame",
		"headers", "height", "hreflang",
		"hspace", "ismap", "label", "lang",
		"maxlength", "media", "method",
		"multiple", "name", "nohref", "noshade",
		"nowrap", "open", "prompt", "readonly", "rel", "rev",
		"rows", "rowspan", "rules", "scope",
		"selected", "shape", "size", "span",
		"start", "summary", "tabindex", "target",
		"title", "type", "usemap", "valign", "value",
		"vspace", "width", "itemprop", "itemscope", "itemtype",
		"data-markdown-generated-content", "data-attr-class",
	}
	generalSafeElements := []string{
		"h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8", "br", "b", "i", "strong", "em", "a", "pre", "code", "img", "tt",
		"div", "ins", "del", "sup", "sub", "p", "ol", "ul", "table", "thead", "tbody", "tfoot", "blockquote", "label",
		"dl", "dt", "dd", "kbd", "q", "samp", "var", "hr", "ruby", "rt", "rp", "li", "tr", "td", "th", "s", "strike", "summary",
		"details", "caption", "figure", "figcaption",
		"abbr", "bdo", "cite", "dfn", "mark", "small", "span", "time", "video", "wbr",
	}
	// FIXME: Need to handle longdesc in img but there is no easy way to do it
	policy.AllowAttrs(generalSafeAttrs...).OnElements(generalSafeElements...)

	// Custom keyword markup
	defaultSanitizer.addSanitizerRules(policy, setting.ExternalSanitizerRules)

	return policy
}

// Sanitize takes a string that contains a HTML fragment or document and applies policy whitelist.
func Sanitize(s string) string {
	return GetDefaultSanitizer().defaultPolicy.Sanitize(s)
}

// SanitizeReader sanitizes a Reader
func SanitizeReader(r io.Reader, renderer string, w io.Writer) error {
	policy, exist := GetDefaultSanitizer().rendererPolicies[renderer]
	if !exist {
		policy = GetDefaultSanitizer().defaultPolicy
	}
	return policy.SanitizeReaderToWriter(r, w)
}
