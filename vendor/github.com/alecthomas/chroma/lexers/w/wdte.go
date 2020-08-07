package w

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// WDTE lexer.
var WDTE = internal.Register(MustNewLexer(
	&Config{
		Name:      "WDTE",
		Filenames: []string{"*.wdte"},
	},
	Rules{
		"root": {
			{`\n`, Text, nil},
			{`\s+`, Text, nil},
			{`\\\n`, Text, nil},
			{`#(.*?)\n`, CommentSingle, nil},
			{`-?[0-9]+`, LiteralNumberInteger, nil},
			{`-?[0-9]*\.[0-9]+`, LiteralNumberFloat, nil},
			{`"[^"]*"`, LiteralString, nil},
			{`'[^']*'`, LiteralString, nil},
			{Words(``, `\b`, `switch`, `default`, `memo`), KeywordReserved, nil},
			{`{|}|;|->|=>|\(|\)|\[|\]|\.`, Operator, nil},
			{`[^{};()[\].\s]+`, NameVariable, nil},
		},
	},
))
