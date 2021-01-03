package d

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// Diff lexer.
var Diff = internal.Register(MustNewLexer(
	&Config{
		Name:      "Diff",
		Aliases:   []string{"diff", "udiff"},
		EnsureNL:  true,
		Filenames: []string{"*.diff", "*.patch"},
		MimeTypes: []string{"text/x-diff", "text/x-patch"},
	},
	Rules{
		"root": {
			{` .*\n`, Text, nil},
			{`\+.*\n`, GenericInserted, nil},
			{`-.*\n`, GenericDeleted, nil},
			{`!.*\n`, GenericStrong, nil},
			{`@.*\n`, GenericSubheading, nil},
			{`([Ii]ndex|diff).*\n`, GenericHeading, nil},
			{`=.*\n`, GenericHeading, nil},
			{`.*\n`, Text, nil},
		},
	},
))
