// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

// Package xurls extracts urls from plain text using regular expressions.
package xurls

import (
	"bytes"
	"regexp"
)

//go:generate go run generate/tldsgen/main.go
//go:generate go run generate/schemesgen/main.go

const (
	letter    = `\p{L}`
	mark      = `\p{M}`
	number    = `\p{N}`
	iriChar   = letter + mark + number
	currency  = `\p{Sc}`
	otherSymb = `\p{So}`
	endChar   = iriChar + `/\-_+&~%=#` + currency + otherSymb
	otherPunc = `\p{Po}`
	midChar   = endChar + "_*" + otherPunc
	wellParen = `\([` + midChar + `]*(\([` + midChar + `]*\)[` + midChar + `]*)*\)`
	wellBrack = `\[[` + midChar + `]*(\[[` + midChar + `]*\][` + midChar + `]*)*\]`
	wellBrace = `\{[` + midChar + `]*(\{[` + midChar + `]*\}[` + midChar + `]*)*\}`
	wellAll   = wellParen + `|` + wellBrack + `|` + wellBrace
	pathCont  = `([` + midChar + `]*(` + wellAll + `|[` + endChar + `])+)+`

	iri      = `[` + iriChar + `]([` + iriChar + `\-]*[` + iriChar + `])?`
	domain   = `(` + iri + `\.)+`
	octet    = `(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9][0-9]|[0-9])`
	ipv4Addr = `\b` + octet + `\.` + octet + `\.` + octet + `\.` + octet + `\b`
	ipv6Addr = `([0-9a-fA-F]{1,4}:([0-9a-fA-F]{1,4}:([0-9a-fA-F]{1,4}:([0-9a-fA-F]{1,4}:([0-9a-fA-F]{1,4}:[0-9a-fA-F]{0,4}|:[0-9a-fA-F]{1,4})?|(:[0-9a-fA-F]{1,4}){0,2})|(:[0-9a-fA-F]{1,4}){0,3})|(:[0-9a-fA-F]{1,4}){0,4})|:(:[0-9a-fA-F]{1,4}){0,5})((:[0-9a-fA-F]{1,4}){2}|:(25[0-5]|(2[0-4]|1[0-9]|[1-9])?[0-9])(\.(25[0-5]|(2[0-4]|1[0-9]|[1-9])?[0-9])){3})|(([0-9a-fA-F]{1,4}:){1,6}|:):[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){7}:`
	ipAddr   = `(` + ipv4Addr + `|` + ipv6Addr + `)`
	port     = `(:[0-9]*)?`
)

// AnyScheme can be passed to StrictMatchingScheme to match any possibly valid
// scheme, and not just the known ones.
var AnyScheme = `([a-zA-Z][a-zA-Z.\-+]*://|` + anyOf(SchemesNoAuthority...) + `:)`

// SchemesNoAuthority is a sorted list of some well-known url schemes that are
// followed by ":" instead of "://".
var SchemesNoAuthority = []string{
	`bitcoin`, // Bitcoin
	`file`,    // Files
	`magnet`,  // Torrent magnets
	`mailto`,  // Mail
	`sms`,     // SMS
	`tel`,     // Telephone
	`xmpp`,    // XMPP
}

func anyOf(strs ...string) string {
	var b bytes.Buffer
	b.WriteByte('(')
	for i, s := range strs {
		if i != 0 {
			b.WriteByte('|')
		}
		b.WriteString(regexp.QuoteMeta(s))
	}
	b.WriteByte(')')
	return b.String()
}

func strictExp() string {
	schemes := `(` + anyOf(Schemes...) + `://|` + anyOf(SchemesNoAuthority...) + `:)`
	return `(?i)` + schemes + `(?-i)` + pathCont
}

func relaxedExp() string {
	punycode := `xn--[a-z0-9-]+`
	knownTLDs := anyOf(append(TLDs, PseudoTLDs...)...)
	site := domain + `(?i)(` + punycode + `|` + knownTLDs + `)(?-i)`
	hostName := `(` + site + `|` + ipAddr + `)`
	webURL := hostName + port + `(/|/` + pathCont + `)?`
	return strictExp() + `|` + webURL
}

// Strict produces a regexp that matches any URL with a scheme in either the
// Schemes or SchemesNoAuthority lists.
func Strict() *regexp.Regexp {
	re := regexp.MustCompile(strictExp())
	re.Longest()
	return re
}

// Relaxed produces a regexp that matches any URL matched by Strict, plus any
// URL with no scheme.
func Relaxed() *regexp.Regexp {
	re := regexp.MustCompile(relaxedExp())
	re.Longest()
	return re
}

// StrictMatchingScheme produces a regexp similar to Strict, but requiring that
// the scheme match the given regular expression. See AnyScheme too.
func StrictMatchingScheme(exp string) (*regexp.Regexp, error) {
	strictMatching := `(?i)(` + exp + `)(?-i)` + pathCont
	re, err := regexp.Compile(strictMatching)
	if err != nil {
		return nil, err
	}
	re.Longest()
	return re, nil
}
