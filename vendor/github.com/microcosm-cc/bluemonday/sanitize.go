// Copyright (c) 2014, David Kitchen <david@buro9.com>
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
//
// * Neither the name of the organisation (Microcosm) nor the names of its
//   contributors may be used to endorse or promote products derived from
//   this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package bluemonday

import (
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	cssparser "github.com/chris-ramon/douceur/parser"
)

var (
	dataAttribute             = regexp.MustCompile("^data-.+")
	dataAttributeXMLPrefix    = regexp.MustCompile("^xml.+")
	dataAttributeInvalidChars = regexp.MustCompile("[A-Z;]+")
	cssUnicodeChar            = regexp.MustCompile(`\\[0-9a-f]{1,6} ?`)
)

// Sanitize takes a string that contains a HTML fragment or document and applies
// the given policy whitelist.
//
// It returns a HTML string that has been sanitized by the policy or an empty
// string if an error has occurred (most likely as a consequence of extremely
// malformed input)
func (p *Policy) Sanitize(s string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}

	return p.sanitize(strings.NewReader(s)).String()
}

// SanitizeBytes takes a []byte that contains a HTML fragment or document and applies
// the given policy whitelist.
//
// It returns a []byte containing the HTML that has been sanitized by the policy
// or an empty []byte if an error has occurred (most likely as a consequence of
// extremely malformed input)
func (p *Policy) SanitizeBytes(b []byte) []byte {
	if len(bytes.TrimSpace(b)) == 0 {
		return b
	}

	return p.sanitize(bytes.NewReader(b)).Bytes()
}

// SanitizeReader takes an io.Reader that contains a HTML fragment or document
// and applies the given policy whitelist.
//
// It returns a bytes.Buffer containing the HTML that has been sanitized by the
// policy. Errors during sanitization will merely return an empty result.
func (p *Policy) SanitizeReader(r io.Reader) *bytes.Buffer {
	return p.sanitize(r)
}

const escapedURLChars = "'<>\"\r"

func escapeUrlComponent(val string) string {
	w := bytes.NewBufferString("")
	i := strings.IndexAny(val, escapedURLChars)
	for i != -1 {
		if _, err := w.WriteString(val[:i]); err != nil {
			return w.String()
		}
		var esc string
		switch val[i] {
		case '\'':
			// "&#39;" is shorter than "&apos;" and apos was not in HTML until HTML5.
			esc = "&#39;"
		case '<':
			esc = "&lt;"
		case '>':
			esc = "&gt;"
		case '"':
			// "&#34;" is shorter than "&quot;".
			esc = "&#34;"
		case '\r':
			esc = "&#13;"
		default:
			panic("unrecognized escape character")
		}
		val = val[i+1:]
		if _, err := w.WriteString(esc); err != nil {
			return w.String()
		}
		i = strings.IndexAny(val, escapedURLChars)
	}
	w.WriteString(val)
	return w.String()
}

func sanitizedUrl(val string) (string, error) {
	u, err := url.Parse(val)
	if err != nil {
		return "", err
	}
	// sanitize the url query params
	sanitizedQueryValues := make(url.Values, 0)
	queryValues := u.Query()
	for k, vals := range queryValues {
		sk := html.EscapeString(k)
		for _, v := range vals {
			sv := v
			sanitizedQueryValues.Add(sk, sv)
		}
	}
	u.RawQuery = sanitizedQueryValues.Encode()
	// u.String() will also sanitize host/scheme/user/pass
	return u.String(), nil
}

func (p *Policy) writeLinkableBuf(buff *bytes.Buffer, token *html.Token) {
	// do not escape multiple query parameters
	tokenBuff := bytes.NewBufferString("")
	tokenBuff.WriteString("<")
	tokenBuff.WriteString(token.Data)
	for _, attr := range token.Attr {
		tokenBuff.WriteByte(' ')
		tokenBuff.WriteString(attr.Key)
		tokenBuff.WriteString(`="`)
		switch attr.Key {
		case "href", "src":
			u, ok := p.validURL(attr.Val)
			if !ok {
				tokenBuff.WriteString(html.EscapeString(attr.Val))
				continue
			}
			u, err := sanitizedUrl(u)
			if err == nil {
				tokenBuff.WriteString(u)
			} else {
				// fallthrough
				tokenBuff.WriteString(html.EscapeString(attr.Val))
			}
		default:
			// re-apply
			tokenBuff.WriteString(html.EscapeString(attr.Val))
		}
		tokenBuff.WriteByte('"')
	}
	if token.Type == html.SelfClosingTagToken {
		tokenBuff.WriteString("/")
	}
	tokenBuff.WriteString(">")
	buff.WriteString(tokenBuff.String())
}

// Performs the actual sanitization process.
func (p *Policy) sanitize(r io.Reader) *bytes.Buffer {

	// It is possible that the developer has created the policy via:
	//   p := bluemonday.Policy{}
	// rather than:
	//   p := bluemonday.NewPolicy()
	// If this is the case, and if they haven't yet triggered an action that
	// would initiliaze the maps, then we need to do that.
	p.init()

	var (
		buff                     bytes.Buffer
		skipElementContent       bool
		skippingElementsCount    int64
		skipClosingTag           bool
		closingTagToSkipStack    []string
		mostRecentlyStartedToken string
	)

	tokenizer := html.NewTokenizer(r)
	for {
		if tokenizer.Next() == html.ErrorToken {
			err := tokenizer.Err()
			if err == io.EOF {
				// End of input means end of processing
				return &buff
			}

			// Raw tokenizer error
			return &bytes.Buffer{}
		}

		token := tokenizer.Token()
		switch token.Type {
		case html.DoctypeToken:

			// DocType is not handled as there is no safe parsing mechanism
			// provided by golang.org/x/net/html for the content, and this can
			// be misused to insert HTML tags that are not then sanitized
			//
			// One might wish to recursively sanitize here using the same policy
			// but I will need to do some further testing before considering
			// this.

		case html.CommentToken:

			// Comments are ignored by default

		case html.StartTagToken:

			mostRecentlyStartedToken = strings.ToLower(token.Data)

			aps, ok := p.elsAndAttrs[token.Data]
			if !ok {
				aa, matched := p.matchRegex(token.Data)
				if !matched {
					if _, ok := p.setOfElementsToSkipContent[token.Data]; ok {
						skipElementContent = true
						skippingElementsCount++
					}
					if p.addSpaces {
						buff.WriteString(" ")
					}
					break
				}
				aps = aa
			}
			if len(token.Attr) != 0 {
				token.Attr = p.sanitizeAttrs(token.Data, token.Attr, aps)
			}

			if len(token.Attr) == 0 {
				if !p.allowNoAttrs(token.Data) {
					skipClosingTag = true
					closingTagToSkipStack = append(closingTagToSkipStack, token.Data)
					if p.addSpaces {
						buff.WriteString(" ")
					}
					break
				}
			}

			if !skipElementContent {
				// do not escape multiple query parameters
				if linkable(token.Data) {
					p.writeLinkableBuf(&buff, &token)
				} else {
					buff.WriteString(token.String())
				}
			}

		case html.EndTagToken:

			if mostRecentlyStartedToken == strings.ToLower(token.Data) {
				mostRecentlyStartedToken = ""
			}

			if skipClosingTag && closingTagToSkipStack[len(closingTagToSkipStack)-1] == token.Data {
				closingTagToSkipStack = closingTagToSkipStack[:len(closingTagToSkipStack)-1]
				if len(closingTagToSkipStack) == 0 {
					skipClosingTag = false
				}
				if p.addSpaces {
					buff.WriteString(" ")
				}
				break
			}
			if _, ok := p.elsAndAttrs[token.Data]; !ok {
				match := false
				for regex := range p.elsMatchingAndAttrs {
					if regex.MatchString(token.Data) {
						skipElementContent = false
						match = true
						break
					}
				}
				if _, ok := p.setOfElementsToSkipContent[token.Data]; ok && !match {
					skippingElementsCount--
					if skippingElementsCount == 0 {
						skipElementContent = false
					}
				}
				if !match {
					if p.addSpaces {
						buff.WriteString(" ")
					}
					break
				}
			}

			if !skipElementContent {
				buff.WriteString(token.String())
			}

		case html.SelfClosingTagToken:

			aps, ok := p.elsAndAttrs[token.Data]
			if !ok {
				aa, matched := p.matchRegex(token.Data)
				if !matched {
					if p.addSpaces && !matched {
						buff.WriteString(" ")
					}
					break
				}
				aps = aa
			}

			if len(token.Attr) != 0 {
				token.Attr = p.sanitizeAttrs(token.Data, token.Attr, aps)
			}

			if len(token.Attr) == 0 && !p.allowNoAttrs(token.Data) {
				if p.addSpaces {
					buff.WriteString(" ")
					break
				}
			}
			if !skipElementContent {
				// do not escape multiple query parameters
				if linkable(token.Data) {
					p.writeLinkableBuf(&buff, &token)
				} else {
					buff.WriteString(token.String())
				}
			}

		case html.TextToken:

			if !skipElementContent {
				switch mostRecentlyStartedToken {
				case "script":
					// not encouraged, but if a policy allows JavaScript we
					// should not HTML escape it as that would break the output
					buff.WriteString(token.Data)
				case "style":
					// not encouraged, but if a policy allows CSS styles we
					// should not HTML escape it as that would break the output
					buff.WriteString(token.Data)
				default:
					// HTML escape the text
					buff.WriteString(token.String())
				}
			}

		default:
			// A token that didn't exist in the html package when we wrote this
			return &bytes.Buffer{}
		}
	}
}

// sanitizeAttrs takes a set of element attribute policies and the global
// attribute policies and applies them to the []html.Attribute returning a set
// of html.Attributes that match the policies
func (p *Policy) sanitizeAttrs(
	elementName string,
	attrs []html.Attribute,
	aps map[string]attrPolicy,
) []html.Attribute {

	if len(attrs) == 0 {
		return attrs
	}

	hasStylePolicies := false
	sps, elementHasStylePolicies := p.elsAndStyles[elementName]
	if len(p.globalStyles) > 0 || (elementHasStylePolicies && len(sps) > 0) {
		hasStylePolicies = true
	}
	// no specific element policy found, look for a pattern match
	if !hasStylePolicies {
		for k, v := range p.elsMatchingAndStyles {
			if k.MatchString(elementName) {
				if len(v) > 0 {
					hasStylePolicies = true
					break
				}
			}
		}
	}

	// Builds a new attribute slice based on the whether the attribute has been
	// whitelisted explicitly or globally.
	cleanAttrs := []html.Attribute{}
	for _, htmlAttr := range attrs {
		if p.allowDataAttributes {
			// If we see a data attribute, let it through.
			if isDataAttribute(htmlAttr.Key) {
				cleanAttrs = append(cleanAttrs, htmlAttr)
				continue
			}
		}
		// Is this a "style" attribute, and if so, do we need to sanitize it?
		if htmlAttr.Key == "style" && hasStylePolicies {
			htmlAttr = p.sanitizeStyles(htmlAttr, elementName)
			if htmlAttr.Val == "" {
				// We've sanitized away any and all styles; don't bother to
				// output the style attribute (even if it's allowed)
				continue
			} else {
				cleanAttrs = append(cleanAttrs, htmlAttr)
				continue
			}
		}

		// Is there an element specific attribute policy that applies?
		if ap, ok := aps[htmlAttr.Key]; ok {
			if ap.regexp != nil {
				if ap.regexp.MatchString(htmlAttr.Val) {
					cleanAttrs = append(cleanAttrs, htmlAttr)
					continue
				}
			} else {
				cleanAttrs = append(cleanAttrs, htmlAttr)
				continue
			}
		}

		// Is there a global attribute policy that applies?
		if ap, ok := p.globalAttrs[htmlAttr.Key]; ok {

			if ap.regexp != nil {
				if ap.regexp.MatchString(htmlAttr.Val) {
					cleanAttrs = append(cleanAttrs, htmlAttr)
				}
			} else {
				cleanAttrs = append(cleanAttrs, htmlAttr)
			}
		}
	}

	if len(cleanAttrs) == 0 {
		// If nothing was allowed, let's get out of here
		return cleanAttrs
	}
	// cleanAttrs now contains the attributes that are permitted

	if linkable(elementName) {
		if p.requireParseableURLs {
			// Ensure URLs are parseable:
			// - a.href
			// - area.href
			// - link.href
			// - blockquote.cite
			// - q.cite
			// - img.src
			// - script.src
			tmpAttrs := []html.Attribute{}
			for _, htmlAttr := range cleanAttrs {
				switch elementName {
				case "a", "area", "link":
					if htmlAttr.Key == "href" {
						if u, ok := p.validURL(htmlAttr.Val); ok {
							htmlAttr.Val = u
							tmpAttrs = append(tmpAttrs, htmlAttr)
						}
						break
					}
					tmpAttrs = append(tmpAttrs, htmlAttr)
				case "blockquote", "q":
					if htmlAttr.Key == "cite" {
						if u, ok := p.validURL(htmlAttr.Val); ok {
							htmlAttr.Val = u
							tmpAttrs = append(tmpAttrs, htmlAttr)
						}
						break
					}
					tmpAttrs = append(tmpAttrs, htmlAttr)
				case "img", "script":
					if htmlAttr.Key == "src" {
						if u, ok := p.validURL(htmlAttr.Val); ok {
							htmlAttr.Val = u
							tmpAttrs = append(tmpAttrs, htmlAttr)
						}
						break
					}
					tmpAttrs = append(tmpAttrs, htmlAttr)
				default:
					tmpAttrs = append(tmpAttrs, htmlAttr)
				}
			}
			cleanAttrs = tmpAttrs
		}

		if (p.requireNoFollow ||
			p.requireNoFollowFullyQualifiedLinks ||
			p.requireNoReferrer ||
			p.requireNoReferrerFullyQualifiedLinks ||
			p.addTargetBlankToFullyQualifiedLinks) &&
			len(cleanAttrs) > 0 {

			// Add rel="nofollow" if a "href" exists
			switch elementName {
			case "a", "area", "link":
				var hrefFound bool
				var externalLink bool
				for _, htmlAttr := range cleanAttrs {
					if htmlAttr.Key == "href" {
						hrefFound = true

						u, err := url.Parse(htmlAttr.Val)
						if err != nil {
							continue
						}
						if u.Host != "" {
							externalLink = true
						}

						continue
					}
				}

				if hrefFound {
					var (
						noFollowFound    bool
						noReferrerFound  bool
						targetBlankFound bool
					)

					addNoFollow := (p.requireNoFollow ||
						externalLink && p.requireNoFollowFullyQualifiedLinks)

					addNoReferrer := (p.requireNoReferrer ||
						externalLink && p.requireNoReferrerFullyQualifiedLinks)

					addTargetBlank := (externalLink &&
						p.addTargetBlankToFullyQualifiedLinks)

					tmpAttrs := []html.Attribute{}
					for _, htmlAttr := range cleanAttrs {

						var appended bool
						if htmlAttr.Key == "rel" && (addNoFollow || addNoReferrer) {

							if addNoFollow && !strings.Contains(htmlAttr.Val, "nofollow") {
								htmlAttr.Val += " nofollow"
							}
							if addNoReferrer && !strings.Contains(htmlAttr.Val, "noreferrer") {
								htmlAttr.Val += " noreferrer"
							}
							noFollowFound = addNoFollow
							noReferrerFound = addNoReferrer
							tmpAttrs = append(tmpAttrs, htmlAttr)
							appended = true
						}

						if elementName == "a" && htmlAttr.Key == "target" {
							if htmlAttr.Val == "_blank" {
								targetBlankFound = true
							}
							if addTargetBlank && !targetBlankFound {
								htmlAttr.Val = "_blank"
								targetBlankFound = true
								tmpAttrs = append(tmpAttrs, htmlAttr)
								appended = true
							}
						}

						if !appended {
							tmpAttrs = append(tmpAttrs, htmlAttr)
						}
					}
					if noFollowFound || noReferrerFound || targetBlankFound {
						cleanAttrs = tmpAttrs
					}

					if (addNoFollow && !noFollowFound) || (addNoReferrer && !noReferrerFound) {
						rel := html.Attribute{}
						rel.Key = "rel"
						if addNoFollow {
							rel.Val = "nofollow"
						}
						if addNoReferrer {
							if rel.Val != "" {
								rel.Val += " "
							}
							rel.Val += "noreferrer"
						}
						cleanAttrs = append(cleanAttrs, rel)
					}

					if elementName == "a" && addTargetBlank && !targetBlankFound {
						rel := html.Attribute{}
						rel.Key = "target"
						rel.Val = "_blank"
						targetBlankFound = true
						cleanAttrs = append(cleanAttrs, rel)
					}

					if targetBlankFound {
						// target="_blank" has a security risk that allows the
						// opened window/tab to issue JavaScript calls against
						// window.opener, which in effect allow the destination
						// of the link to control the source:
						// https://dev.to/ben/the-targetblank-vulnerability-by-example
						//
						// To mitigate this risk, we need to add a specific rel
						// attribute if it is not already present.
						// rel="noopener"
						//
						// Unfortunately this is processing the rel twice (we
						// already looked at it earlier ^^) as we cannot be sure
						// of the ordering of the href and rel, and whether we
						// have fully satisfied that we need to do this. This
						// double processing only happens *if* target="_blank"
						// is true.
						var noOpenerAdded bool
						tmpAttrs := []html.Attribute{}
						for _, htmlAttr := range cleanAttrs {
							var appended bool
							if htmlAttr.Key == "rel" {
								if strings.Contains(htmlAttr.Val, "noopener") {
									noOpenerAdded = true
									tmpAttrs = append(tmpAttrs, htmlAttr)
								} else {
									htmlAttr.Val += " noopener"
									noOpenerAdded = true
									tmpAttrs = append(tmpAttrs, htmlAttr)
								}

								appended = true
							}
							if !appended {
								tmpAttrs = append(tmpAttrs, htmlAttr)
							}
						}
						if noOpenerAdded {
							cleanAttrs = tmpAttrs
						} else {
							// rel attr was not found, or else noopener would
							// have been added already
							rel := html.Attribute{}
							rel.Key = "rel"
							rel.Val = "noopener"
							cleanAttrs = append(cleanAttrs, rel)
						}

					}
				}
			default:
			}
		}
	}

	return cleanAttrs
}

func (p *Policy) sanitizeStyles(attr html.Attribute, elementName string) html.Attribute {
	sps := p.elsAndStyles[elementName]
	if len(sps) == 0 {
		sps = map[string]stylePolicy{}
		// check for any matching elements, if we don't already have a policy found
		// if multiple matches are found they will be overwritten, it's best
		// to not have overlapping matchers
		for regex, policies := range p.elsMatchingAndStyles {
			if regex.MatchString(elementName) {
				for k, v := range policies {
					sps[k] = v
				}
			}
		}
	}

	//Add semi-colon to end to fix parsing issue
	if len(attr.Val) > 0 && attr.Val[len(attr.Val)-1] != ';' {
		attr.Val = attr.Val + ";"
	}
	decs, err := cssparser.ParseDeclarations(attr.Val)
	if err != nil {
		attr.Val = ""
		return attr
	}
	clean := []string{}
	prefixes := []string{"-webkit-", "-moz-", "-ms-", "-o-", "mso-", "-xv-", "-atsc-", "-wap-", "-khtml-", "prince-", "-ah-", "-hp-", "-ro-", "-rim-", "-tc-"}

	for _, dec := range decs {
		addedProperty := false
		tempProperty := strings.ToLower(dec.Property)
		tempValue := removeUnicode(strings.ToLower(dec.Value))
		for _, i := range prefixes {
			tempProperty = strings.TrimPrefix(tempProperty, i)
		}
		if sp, ok := sps[tempProperty]; ok {
			if sp.handler != nil {
				if sp.handler(tempValue) {
					clean = append(clean, dec.Property+": "+dec.Value)
					addedProperty = true
				}
			} else if len(sp.enum) > 0 {
				if stringInSlice(tempValue, sp.enum) {
					clean = append(clean, dec.Property+": "+dec.Value)
					addedProperty = true
				}
			} else if sp.regexp != nil {
				if sp.regexp.MatchString(tempValue) {
					clean = append(clean, dec.Property+": "+dec.Value)
					addedProperty = true
				}
				continue
			}
		}
		if sp, ok := p.globalStyles[tempProperty]; ok && !addedProperty {
			if sp.handler != nil {
				if sp.handler(tempValue) {
					clean = append(clean, dec.Property+": "+dec.Value)
				}
			} else if len(sp.enum) > 0 {
				if stringInSlice(tempValue, sp.enum) {
					clean = append(clean, dec.Property+": "+dec.Value)
				}
			} else if sp.regexp != nil {
				if sp.regexp.MatchString(tempValue) {
					clean = append(clean, dec.Property+": "+dec.Value)
				}
				continue
			}
		}
	}
	if len(clean) > 0 {
		attr.Val = strings.Join(clean, "; ")
	} else {
		attr.Val = ""
	}
	return attr
}

func (p *Policy) allowNoAttrs(elementName string) bool {
	_, ok := p.setOfElementsAllowedWithoutAttrs[elementName]
	if !ok {
		for _, r := range p.setOfElementsMatchingAllowedWithoutAttrs {
			if r.MatchString(elementName) {
				ok = true
				break
			}
		}
	}
	return ok
}

func (p *Policy) validURL(rawurl string) (string, bool) {
	if p.requireParseableURLs {
		// URLs are valid if when space is trimmed the URL is valid
		rawurl = strings.TrimSpace(rawurl)

		// URLs cannot contain whitespace, unless it is a data-uri
		if (strings.Contains(rawurl, " ") ||
			strings.Contains(rawurl, "\t") ||
			strings.Contains(rawurl, "\n")) &&
			!strings.HasPrefix(rawurl, `data:`) {
			return "", false
		}

		// URLs are valid if they parse
		u, err := url.Parse(rawurl)
		if err != nil {
			return "", false
		}

		if u.Scheme != "" {

			urlPolicy, ok := p.allowURLSchemes[u.Scheme]
			if !ok {
				return "", false

			}

			if urlPolicy == nil || urlPolicy(u) == true {
				return u.String(), true
			}

			return "", false
		}

		if p.allowRelativeURLs {
			if u.String() != "" {
				return u.String(), true
			}
		}

		return "", false
	}

	return rawurl, true
}

func linkable(elementName string) bool {
	switch elementName {
	case "a", "area", "blockquote", "img", "link", "script":
		return true
	default:
		return false
	}
}

// stringInSlice returns true if needle exists in haystack
func stringInSlice(needle string, haystack []string) bool {
	for _, straw := range haystack {
		if strings.ToLower(straw) == strings.ToLower(needle) {
			return true
		}
	}
	return false
}

func isDataAttribute(val string) bool {
	if !dataAttribute.MatchString(val) {
		return false
	}
	rest := strings.Split(val, "data-")
	if len(rest) == 1 {
		return false
	}
	// data-xml* is invalid.
	if dataAttributeXMLPrefix.MatchString(rest[1]) {
		return false
	}
	// no uppercase or semi-colons allowed.
	if dataAttributeInvalidChars.MatchString(rest[1]) {
		return false
	}
	return true
}

func removeUnicode(value string) string {
	substitutedValue := value
	currentLoc := cssUnicodeChar.FindStringIndex(substitutedValue)
	for currentLoc != nil {

		character := substitutedValue[currentLoc[0]+1 : currentLoc[1]]
		character = strings.TrimSpace(character)
		if len(character) < 4 {
			character = strings.Repeat("0", 4-len(character)) + character
		} else {
			for len(character) > 4 {
				if character[0] != '0' {
					character = ""
					break
				} else {
					character = character[1:]
				}
			}
		}
		character = "\\u" + character
		translatedChar, err := strconv.Unquote(`"` + character + `"`)
		translatedChar = strings.TrimSpace(translatedChar)
		if err != nil {
			return ""
		}
		substitutedValue = substitutedValue[0:currentLoc[0]] + translatedChar + substitutedValue[currentLoc[1]:]
		currentLoc = cssUnicodeChar.FindStringIndex(substitutedValue)
	}
	return substitutedValue
}

func (p *Policy) matchRegex(elementName string) (map[string]attrPolicy, bool) {
	aps := make(map[string]attrPolicy, 0)
	matched := false
	for regex, attrs := range p.elsMatchingAndAttrs {
		if regex.MatchString(elementName) {
			matched = true
			for k, v := range attrs {
				aps[k] = v
			}
		}
	}
	return aps, matched
}
