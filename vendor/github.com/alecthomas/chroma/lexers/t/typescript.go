package t

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// TypeScript lexer.
var TypeScript = internal.Register(MustNewLexer(
	&Config{
		Name:      "TypeScript",
		Aliases:   []string{"ts", "tsx", "typescript"},
		Filenames: []string{"*.ts", "*.tsx"},
		MimeTypes: []string{"text/x-typescript"},
		DotAll:    true,
		EnsureNL:  true,
	},
	Rules{
		"commentsandwhitespace": {
			{`\s+`, Text, nil},
			{`<!--`, Comment, nil},
			{`//.*?\n`, CommentSingle, nil},
			{`/\*.*?\*/`, CommentMultiline, nil},
		},
		"slashstartsregex": {
			Include("commentsandwhitespace"),
			{`/(\\.|[^[/\\\n]|\[(\\.|[^\]\\\n])*])+/([gim]+\b|\B)`, LiteralStringRegex, Pop(1)},
			{`(?=/)`, Text, Push("#pop", "badregex")},
			Default(Pop(1)),
		},
		"badregex": {
			{`\n`, Text, Pop(1)},
		},
		"root": {
			Include("jsx"),
			{`^(?=\s|/|<!--)`, Text, Push("slashstartsregex")},
			Include("commentsandwhitespace"),
			{`\+\+|--|~|&&|\?|:|\|\||\\(?=\n)|(<<|>>>?|==?|!=?|[-<>+*%&|^/])=?`, Operator, Push("slashstartsregex")},
			{`[{(\[;,]`, Punctuation, Push("slashstartsregex")},
			{`[})\].]`, Punctuation, nil},
			{`(for|in|while|do|break|return|continue|switch|case|default|if|else|throw|try|catch|finally|new|delete|typeof|instanceof|void|this)\b`, Keyword, Push("slashstartsregex")},
			{`(var|let|with|function)\b`, KeywordDeclaration, Push("slashstartsregex")},
			{`(abstract|boolean|byte|char|class|const|debugger|double|enum|export|extends|final|float|goto|implements|import|int|interface|long|native|package|private|protected|public|short|static|super|synchronized|throws|transient|volatile)\b`, KeywordReserved, nil},
			{`(true|false|null|NaN|Infinity|undefined)\b`, KeywordConstant, nil},
			{`(Array|Boolean|Date|Error|Function|Math|netscape|Number|Object|Packages|RegExp|String|sun|decodeURI|decodeURIComponent|encodeURI|encodeURIComponent|Error|eval|isFinite|isNaN|parseFloat|parseInt|document|this|window)\b`, NameBuiltin, nil},
			{`\b(module)(\s*)(\s*[\w?.$][\w?.$]*)(\s*)`, ByGroups(KeywordReserved, Text, NameOther, Text), Push("slashstartsregex")},
			{`\b(string|bool|number)\b`, KeywordType, nil},
			{`\b(constructor|declare|interface|as|AS)\b`, KeywordReserved, nil},
			{`(super)(\s*)(\([\w,?.$\s]+\s*\))`, ByGroups(KeywordReserved, Text), Push("slashstartsregex")},
			{`([a-zA-Z_?.$][\w?.$]*)\(\) \{`, NameOther, Push("slashstartsregex")},
			{`([\w?.$][\w?.$]*)(\s*:\s*)([\w?.$][\w?.$]*)`, ByGroups(NameOther, Text, KeywordType), nil},
			{`[$a-zA-Z_]\w*`, NameOther, nil},
			{`[0-9][0-9]*\.[0-9]+([eE][0-9]+)?[fd]?`, LiteralNumberFloat, nil},
			{`0x[0-9a-fA-F]+`, LiteralNumberHex, nil},
			{`[0-9]+`, LiteralNumberInteger, nil},
			{`"(\\\\|\\"|[^"])*"`, LiteralStringDouble, nil},
			{`'(\\\\|\\'|[^'])*'`, LiteralStringSingle, nil},
			{"`", LiteralStringBacktick, Push("interp")},
			{`@\w+`, KeywordDeclaration, nil},
		},
		"interp": {
			{"`", LiteralStringBacktick, Pop(1)},
			{`\\\\`, LiteralStringBacktick, nil},
			{"\\\\`", LiteralStringBacktick, nil},
			{`\$\{`, LiteralStringInterpol, Push("interp-inside")},
			{`\$`, LiteralStringBacktick, nil},
			{"[^`\\\\$]+", LiteralStringBacktick, nil},
		},
		"interp-inside": {
			{`\}`, LiteralStringInterpol, Pop(1)},
			Include("root"),
		},
		"jsx": {
			{`(<)(/?)(>)`, ByGroups(Punctuation, Punctuation, Punctuation), nil},
			{`(<)([\w\.]+)`, ByGroups(Punctuation, NameTag), Push("tag")},
			{`(<)(/)([\w\.]*)(>)`, ByGroups(Punctuation, Punctuation, NameTag, Punctuation), nil},
		},
		"tag": {
			{`\s+`, Text, nil},
			{`([\w]+\s*)(=)(\s*)`, ByGroups(NameAttribute, Operator, Text), Push("attr")},
			{`[{}]+`, Punctuation, nil},
			{`[\w\.]+`, NameAttribute, nil},
			{`(/?)(\s*)(>)`, ByGroups(Punctuation, Text, Punctuation), Pop(1)},
		},
		"attr": {
			{`{`, Punctuation, Push("expression")},
			{`".*?"`, LiteralString, Pop(1)},
			{`'.*?'`, LiteralString, Pop(1)},
			Default(Pop(1)),
		},
		"expression": {
			{`{`, Punctuation, Push()},
			{`}`, Punctuation, Pop(1)},
			Include("root"),
		},
	},
))
