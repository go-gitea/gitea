package s

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

var SYSTEMD = internal.Register(MustNewLexer(
	&Config{
		Name:      "SYSTEMD",
		Aliases:   []string{"systemd"},
		Filenames: []string{"*.service"},
		MimeTypes: []string{"text/plain"},
	},
	Rules{
		"root": {
			{`\s+`, Text, nil},
			{`[;#].*`, Comment, nil},
			{`\[.*?\]$`, Keyword, nil},
			{`(.*?)(=)(.*)(\\\n)`, ByGroups(NameAttribute, Operator, LiteralString, Text), Push("continuation")},
			{`(.*?)(=)(.*)`, ByGroups(NameAttribute, Operator, LiteralString), nil},
		},
		"continuation": {
			{`(.*?)(\\\n)`, ByGroups(LiteralString, Text), nil},
			{`(.*)`, LiteralString, Pop(1)},
		},
	},
))
