package y

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

var YAML = internal.Register(MustNewLexer(
	&Config{
		Name:      "YAML",
		Aliases:   []string{"yaml"},
		Filenames: []string{"*.yaml", "*.yml"},
		MimeTypes: []string{"text/x-yaml"},
	},
	Rules{
		"root": {
			Include("whitespace"),
			{`^---`, Text, nil},
			{`[\n?]?\s*- `, Text, nil},
			{`#.*$`, Comment, nil},
			{`!![^\s]+`, CommentPreproc, nil},
			{`&[^\s]+`, CommentPreproc, nil},
			{`\*[^\s]+`, CommentPreproc, nil},
			{`^%include\s+[^\n\r]+`, CommentPreproc, nil},
			{`([>|+-]\s+)(\s+)((?:(?:.*?$)(?:[\n\r]*?)?)*)`, ByGroups(StringDoc, StringDoc, StringDoc), nil},
			Include("key"),
			Include("value"),
			{`[?:,\[\]]`, Punctuation, nil},
			{`.`, Text, nil},
		},
		"value": {
			{Words(``, `\b`, "true", "false", "null"), KeywordConstant, nil},
			{`"(?:\\.|[^"])*"`, StringDouble, nil},
			{`'(?:\\.|[^'])*'`, StringSingle, nil},
			{`\d\d\d\d-\d\d-\d\d([T ]\d\d:\d\d:\d\d(\.\d+)?(Z|\s+[-+]\d+)?)?`, LiteralDate, nil},
			{`\b[+\-]?(0x[\da-f]+|0o[0-7]+|(\d+\.?\d*|\.?\d+)(e[\+\-]?\d+)?|\.inf|\.nan)\b`, Number, nil},
			{`\b[\w]+\b`, Text, nil},
		},
		"key": {
			{`"[^"\n].*": `, Keyword, nil},
			{`(-)( )([^"\n{]*)(:)( )`, ByGroups(Punctuation, Whitespace, Keyword, Punctuation, Whitespace), nil},
			{`([^"\n{]*)(:)( )`, ByGroups(Keyword, Punctuation, Whitespace), nil},
			{`([^"\n{]*)(:)(\n)`, ByGroups(Keyword, Punctuation, Whitespace), nil},
		},
		"whitespace": {
			{`\s+`, Whitespace, nil},
			{`\n+`, Whitespace, nil},
		},
	},
))
