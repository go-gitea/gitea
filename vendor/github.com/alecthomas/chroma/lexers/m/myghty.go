package m

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
	. "github.com/alecthomas/chroma/lexers/p" // nolint
)

// Myghty lexer.
var Myghty = internal.Register(MustNewLexer(
	&Config{
		Name:      "Myghty",
		Aliases:   []string{"myghty"},
		Filenames: []string{"*.myt", "autodelegate"},
		MimeTypes: []string{"application/x-myghty"},
	},
	Rules{
		"root": {
			{`\s+`, Text, nil},
			{`(<%(?:def|method))(\s*)(.*?)(>)(.*?)(</%\2\s*>)(?s)`, ByGroups(NameTag, Text, NameFunction, NameTag, UsingSelf("root"), NameTag), nil},
			{`(<%\w+)(.*?)(>)(.*?)(</%\2\s*>)(?s)`, ByGroups(NameTag, NameFunction, NameTag, Using(Python), NameTag), nil},
			{`(<&[^|])(.*?)(,.*?)?(&>)`, ByGroups(NameTag, NameFunction, Using(Python), NameTag), nil},
			{`(<&\|)(.*?)(,.*?)?(&>)(?s)`, ByGroups(NameTag, NameFunction, Using(Python), NameTag), nil},
			{`</&>`, NameTag, nil},
			{`(<%!?)(.*?)(%>)(?s)`, ByGroups(NameTag, Using(Python), NameTag), nil},
			{`(?<=^)#[^\n]*(\n|\Z)`, Comment, nil},
			{`(?<=^)(%)([^\n]*)(\n|\Z)`, ByGroups(NameTag, Using(Python), Other), nil},
			{`(?sx)
                 (.+?)               # anything, followed by:
                 (?:
                  (?<=\n)(?=[%#]) |  # an eval or comment line
                  (?=</?[%&]) |      # a substitution or block or
                                     # call start or end
                                     # - don't consume
                  (\\\n) |           # an escaped newline
                  \Z                 # end of string
                 )`, ByGroups(Other, Operator), nil},
		},
	},
))
