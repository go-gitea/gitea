package circular

import (
	"strings"

	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/h"
	"github.com/alecthomas/chroma/lexers/internal"
)

// PHTML lexer is PHP in HTML.
var PHTML = internal.Register(DelegatingLexer(h.HTML, MustNewLexer(
	&Config{
		Name:            "PHTML",
		Aliases:         []string{"phtml"},
		Filenames:       []string{"*.phtml"},
		MimeTypes:       []string{"application/x-php", "application/x-httpd-php", "application/x-httpd-php3", "application/x-httpd-php4", "application/x-httpd-php5"},
		DotAll:          true,
		CaseInsensitive: true,
		EnsureNL:        true,
	},
	Rules{
		"root": {
			{`<\?(php)?`, CommentPreproc, Push("php")},
			{`[^<]+`, Other, nil},
			{`<`, Other, nil},
		},
	}.Merge(phpCommonRules),
).SetAnalyser(func(text string) float32 {
	if strings.Contains(text, "<?php") {
		return 0.5
	}
	return 0.0
})))
