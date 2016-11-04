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
	"strings"

	"golang.org/x/net/html"
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

			if p.allowDocType {
				buff.WriteString(token.String())
			}

		case html.CommentToken:

			// Comments are ignored by default

		case html.StartTagToken:

			mostRecentlyStartedToken = token.Data

			aps, ok := p.elsAndAttrs[token.Data]
			if !ok {
				if _, ok := p.setOfElementsToSkipContent[token.Data]; ok {
					skipElementContent = true
					skippingElementsCount++
				}
				if p.addSpaces {
					buff.WriteString(" ")
				}
				break
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
				buff.WriteString(token.String())
			}

		case html.EndTagToken:

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
				if _, ok := p.setOfElementsToSkipContent[token.Data]; ok {
					skippingElementsCount--
					if skippingElementsCount == 0 {
						skipElementContent = false
					}
				}
				if p.addSpaces {
					buff.WriteString(" ")
				}
				break
			}

			if !skipElementContent {
				buff.WriteString(token.String())
			}

		case html.SelfClosingTagToken:

			aps, ok := p.elsAndAttrs[token.Data]
			if !ok {
				if p.addSpaces {
					buff.WriteString(" ")
				}
				break
			}

			if len(token.Attr) != 0 {
				token.Attr = p.sanitizeAttrs(token.Data, token.Attr, aps)
			}

			if len(token.Attr) == 0 && !p.allowNoAttrs(token.Data) {
				if p.addSpaces {
					buff.WriteString(" ")
				}
				break
			}

			if !skipElementContent {
				buff.WriteString(token.String())
			}

		case html.TextToken:

			if !skipElementContent {
				switch strings.ToLower(mostRecentlyStartedToken) {
				case "javascript":
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

	// Builds a new attribute slice based on the whether the attribute has been
	// whitelisted explicitly or globally.
	cleanAttrs := []html.Attribute{}
	for _, htmlAttr := range attrs {
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
						targetBlankFound bool
					)

					addNoFollow := (p.requireNoFollow ||
						externalLink && p.requireNoFollowFullyQualifiedLinks)

					addTargetBlank := (externalLink &&
						p.addTargetBlankToFullyQualifiedLinks)

					tmpAttrs := []html.Attribute{}
					for _, htmlAttr := range cleanAttrs {

						var appended bool
						if htmlAttr.Key == "rel" && addNoFollow {

							if strings.Contains(htmlAttr.Val, "nofollow") {
								noFollowFound = true
								tmpAttrs = append(tmpAttrs, htmlAttr)
								appended = true
							} else {
								htmlAttr.Val += " nofollow"
								noFollowFound = true
								tmpAttrs = append(tmpAttrs, htmlAttr)
								appended = true
							}
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
					if noFollowFound || targetBlankFound {
						cleanAttrs = tmpAttrs
					}

					if addNoFollow && !noFollowFound {
						rel := html.Attribute{}
						rel.Key = "rel"
						rel.Val = "nofollow"
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

func (p *Policy) allowNoAttrs(elementName string) bool {
	_, ok := p.setOfElementsAllowedWithoutAttrs[elementName]
	return ok
}

func (p *Policy) validURL(rawurl string) (string, bool) {
	if p.requireParseableURLs {
		// URLs do not contain whitespace
		if strings.Contains(rawurl, " ") ||
			strings.Contains(rawurl, "\t") ||
			strings.Contains(rawurl, "\n") {
			return "", false
		}

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
