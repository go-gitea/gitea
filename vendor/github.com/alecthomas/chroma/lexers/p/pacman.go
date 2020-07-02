package p

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// Pacmanconf lexer.
var Pacmanconf = internal.Register(MustNewLexer(
	&Config{
		Name:      "PacmanConf",
		Aliases:   []string{"pacmanconf"},
		Filenames: []string{"pacman.conf"},
		MimeTypes: []string{},
	},
	Rules{
		"root": {
			{`#.*$`, CommentSingle, nil},
			{`^\s*\[.*?\]\s*$`, Keyword, nil},
			{`(\w+)(\s*)(=)`, ByGroups(NameAttribute, Text, Operator), nil},
			{`^(\s*)(\w+)(\s*)$`, ByGroups(Text, NameAttribute, Text), nil},
			{Words(``, `\b`, `$repo`, `$arch`, `%o`, `%u`), NameVariable, nil},
			{`.`, Text, nil},
		},
	},
))
