package f

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// Fish lexer.
var Fish = internal.Register(MustNewLexer(
	&Config{
		Name:      "Fish",
		Aliases:   []string{"fish", "fishshell"},
		Filenames: []string{"*.fish", "*.load"},
		MimeTypes: []string{"application/x-fish"},
	},
	Rules{
		"root": {
			Include("basic"),
			Include("data"),
			Include("interp"),
		},
		"interp": {
			{`\$\(\(`, Keyword, Push("math")},
			{`\(`, Keyword, Push("paren")},
			{`\$#?(\w+|.)`, NameVariable, nil},
		},
		"basic": {
			{`\b(begin|end|if|else|while|break|for|in|return|function|block|case|continue|switch|not|and|or|set|echo|exit|pwd|true|false|cd|count|test)(\s*)\b`, ByGroups(Keyword, Text), nil},
			{`\b(alias|bg|bind|breakpoint|builtin|command|commandline|complete|contains|dirh|dirs|emit|eval|exec|fg|fish|fish_config|fish_indent|fish_pager|fish_prompt|fish_right_prompt|fish_update_completions|fishd|funced|funcsave|functions|help|history|isatty|jobs|math|mimedb|nextd|open|popd|prevd|psub|pushd|random|read|set_color|source|status|trap|type|ulimit|umask|vared|fc|getopts|hash|kill|printf|time|wait)\s*\b(?!\.)`, NameBuiltin, nil},
			{`#.*\n`, Comment, nil},
			{`\\[\w\W]`, LiteralStringEscape, nil},
			{`(\b\w+)(\s*)(=)`, ByGroups(NameVariable, Text, Operator), nil},
			{`[\[\]()=]`, Operator, nil},
			{`<<-?\s*(\'?)\\?(\w+)[\w\W]+?\2`, LiteralString, nil},
		},
		"data": {
			{`(?s)\$?"(\\\\|\\[0-7]+|\\.|[^"\\$])*"`, LiteralStringDouble, nil},
			{`"`, LiteralStringDouble, Push("string")},
			{`(?s)\$'(\\\\|\\[0-7]+|\\.|[^'\\])*'`, LiteralStringSingle, nil},
			{`(?s)'.*?'`, LiteralStringSingle, nil},
			{`;`, Punctuation, nil},
			{`&|\||\^|<|>`, Operator, nil},
			{`\s+`, Text, nil},
			{`\d+(?= |\Z)`, LiteralNumber, nil},
			{"[^=\\s\\[\\]{}()$\"\\'`\\\\<&|;]+", Text, nil},
		},
		"string": {
			{`"`, LiteralStringDouble, Pop(1)},
			{`(?s)(\\\\|\\[0-7]+|\\.|[^"\\$])+`, LiteralStringDouble, nil},
			Include("interp"),
		},
		"paren": {
			{`\)`, Keyword, Pop(1)},
			Include("root"),
		},
		"math": {
			{`\)\)`, Keyword, Pop(1)},
			{`[-+*/%^|&]|\*\*|\|\|`, Operator, nil},
			{`\d+#\d+`, LiteralNumber, nil},
			{`\d+#(?! )`, LiteralNumber, nil},
			{`\d+`, LiteralNumber, nil},
			Include("root"),
		},
	},
))
