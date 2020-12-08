package e

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// Elixir lexer.
var Elixir = internal.Register(MustNewLexer(
	&Config{
		Name:      "Elixir",
		Aliases:   []string{"elixir", "ex", "exs"},
		Filenames: []string{"*.ex", "*.exs"},
		MimeTypes: []string{"text/x-elixir"},
	},
	Rules{
		"root": {
			{`\s+`, Text, nil},
			{`#.*$`, CommentSingle, nil},
			{`(\?)(\\x\{)([\da-fA-F]+)(\})`, ByGroups(LiteralStringChar, LiteralStringEscape, LiteralNumberHex, LiteralStringEscape), nil},
			{`(\?)(\\x[\da-fA-F]{1,2})`, ByGroups(LiteralStringChar, LiteralStringEscape), nil},
			{`(\?)(\\[abdefnrstv])`, ByGroups(LiteralStringChar, LiteralStringEscape), nil},
			{`\?\\?.`, LiteralStringChar, nil},
			{`:::`, LiteralStringSymbol, nil},
			{`::`, Operator, nil},
			{`:(?:\.\.\.|<<>>|%\{\}|%|\{\})`, LiteralStringSymbol, nil},
			{`:(?:(?:\.\.\.|[a-z_]\w*[!?]?)|[A-Z]\w*(?:\.[A-Z]\w*)*|(?:\<\<\<|\>\>\>|\|\|\||\&\&\&|\^\^\^|\~\~\~|\=\=\=|\!\=\=|\~\>\>|\<\~\>|\|\~\>|\<\|\>|\=\=|\!\=|\<\=|\>\=|\&\&|\|\||\<\>|\+\+|\-\-|\|\>|\=\~|\-\>|\<\-|\||\.|\=|\~\>|\<\~|\<|\>|\+|\-|\*|\/|\!|\^|\&))`, LiteralStringSymbol, nil},
			{`:"`, LiteralStringSymbol, Push("string_double_atom")},
			{`:'`, LiteralStringSymbol, Push("string_single_atom")},
			{`((?:\.\.\.|<<>>|%\{\}|%|\{\})|(?:(?:\.\.\.|[a-z_]\w*[!?]?)|[A-Z]\w*(?:\.[A-Z]\w*)*|(?:\<\<\<|\>\>\>|\|\|\||\&\&\&|\^\^\^|\~\~\~|\=\=\=|\!\=\=|\~\>\>|\<\~\>|\|\~\>|\<\|\>|\=\=|\!\=|\<\=|\>\=|\&\&|\|\||\<\>|\+\+|\-\-|\|\>|\=\~|\-\>|\<\-|\||\.|\=|\~\>|\<\~|\<|\>|\+|\-|\*|\/|\!|\^|\&)))(:)(?=\s|\n)`, ByGroups(LiteralStringSymbol, Punctuation), nil},
			{`(fn|do|end|after|else|rescue|catch)\b`, Keyword, nil},
			{`(not|and|or|when|in)\b`, OperatorWord, nil},
			{`(case|cond|for|if|unless|try|receive|raise|quote|unquote|unquote_splicing|throw|super|while)\b`, Keyword, nil},
			{`(def|defp|defmodule|defprotocol|defmacro|defmacrop|defdelegate|defexception|defstruct|defimpl|defcallback)\b`, KeywordDeclaration, nil},
			{`(import|require|use|alias)\b`, KeywordNamespace, nil},
			{`(nil|true|false)\b`, NameConstant, nil},
			{`(_|__MODULE__|__DIR__|__ENV__|__CALLER__)\b`, NamePseudo, nil},
			{`@(?:\.\.\.|[a-z_]\w*[!?]?)`, NameAttribute, nil},
			{`(?:\.\.\.|[a-z_]\w*[!?]?)`, Name, nil},
			{`(%?)([A-Z]\w*(?:\.[A-Z]\w*)*)`, ByGroups(Punctuation, NameClass), nil},
			{`\<\<\<|\>\>\>|\|\|\||\&\&\&|\^\^\^|\~\~\~|\=\=\=|\!\=\=|\~\>\>|\<\~\>|\|\~\>|\<\|\>`, Operator, nil},
			{`\=\=|\!\=|\<\=|\>\=|\&\&|\|\||\<\>|\+\+|\-\-|\|\>|\=\~|\-\>|\<\-|\||\.|\=|\~\>|\<\~`, Operator, nil},
			{`\\\\|\<\<|\>\>|\=\>|\(|\)|\:|\;|\,|\[|\]`, Punctuation, nil},
			{`&\d`, NameEntity, nil},
			{`\<|\>|\+|\-|\*|\/|\!|\^|\&`, Operator, nil},
			{`0b[01](_?[01])*`, LiteralNumberBin, nil},
			{`0o[0-7](_?[0-7])*`, LiteralNumberOct, nil},
			{`0x[\da-fA-F](_?[\dA-Fa-f])*`, LiteralNumberHex, nil},
			{`\d(_?\d)*\.\d(_?\d)*([eE][-+]?\d(_?\d)*)?`, LiteralNumberFloat, nil},
			{`\d(_?\d)*`, LiteralNumberInteger, nil},
			{`"""\s*`, LiteralStringHeredoc, Push("heredoc_double")},
			{`'''\s*$`, LiteralStringHeredoc, Push("heredoc_single")},
			{`"`, LiteralStringDouble, Push("string_double")},
			{`'`, LiteralStringSingle, Push("string_single")},
			Include("sigils"),
			{`%\{`, Punctuation, Push("map_key")},
			{`\{`, Punctuation, Push("tuple")},
		},
		"heredoc_double": {
			{`^\s*"""`, LiteralStringHeredoc, Pop(1)},
			Include("heredoc_interpol"),
		},
		"heredoc_single": {
			{`^\s*'''`, LiteralStringHeredoc, Pop(1)},
			Include("heredoc_interpol"),
		},
		"heredoc_interpol": {
			{`[^#\\\n]+`, LiteralStringHeredoc, nil},
			Include("escapes"),
			{`\\.`, LiteralStringHeredoc, nil},
			{`\n+`, LiteralStringHeredoc, nil},
			Include("interpol"),
		},
		"heredoc_no_interpol": {
			{`[^\\\n]+`, LiteralStringHeredoc, nil},
			{`\\.`, LiteralStringHeredoc, nil},
			{`\n+`, LiteralStringHeredoc, nil},
		},
		"escapes": {
			{`(\\x\{)([\da-fA-F]+)(\})`, ByGroups(LiteralStringEscape, LiteralNumberHex, LiteralStringEscape), nil},
			{`(\\x[\da-fA-F]{1,2})`, LiteralStringEscape, nil},
			{`(\\[abdefnrstv])`, LiteralStringEscape, nil},
		},
		"interpol": {
			{`#\{`, LiteralStringInterpol, Push("interpol_string")},
		},
		"interpol_string": {
			{`\}`, LiteralStringInterpol, Pop(1)},
			Include("root"),
		},
		"map_key": {
			Include("root"),
			{`:`, Punctuation, Push("map_val")},
			{`=>`, Punctuation, Push("map_val")},
			{`\}`, Punctuation, Pop(1)},
		},
		"map_val": {
			Include("root"),
			{`,`, Punctuation, Pop(1)},
			{`(?=\})`, Punctuation, Pop(1)},
		},
		"tuple": {
			Include("root"),
			{`\}`, Punctuation, Pop(1)},
		},
		"string_double": {
			{`[^#"\\]+`, LiteralStringDouble, nil},
			Include("escapes"),
			{`\\.`, LiteralStringDouble, nil},
			{`(")`, ByGroups(LiteralStringDouble), Pop(1)},
			Include("interpol"),
		},
		"string_single": {
			{`[^#'\\]+`, LiteralStringSingle, nil},
			Include("escapes"),
			{`\\.`, LiteralStringSingle, nil},
			{`(')`, ByGroups(LiteralStringSingle), Pop(1)},
			Include("interpol"),
		},
		"string_double_atom": {
			{`[^#"\\]+`, LiteralStringSymbol, nil},
			Include("escapes"),
			{`\\.`, LiteralStringSymbol, nil},
			{`(")`, ByGroups(LiteralStringSymbol), Pop(1)},
			Include("interpol"),
		},
		"string_single_atom": {
			{`[^#'\\]+`, LiteralStringSymbol, nil},
			Include("escapes"),
			{`\\.`, LiteralStringSymbol, nil},
			{`(')`, ByGroups(LiteralStringSymbol), Pop(1)},
			Include("interpol"),
		},
		"sigils": {
			{`(~[a-z])(""")`, ByGroups(LiteralStringOther, LiteralStringHeredoc), Push("triquot-end", "triquot-intp")},
			{`(~[A-Z])(""")`, ByGroups(LiteralStringOther, LiteralStringHeredoc), Push("triquot-end", "triquot-no-intp")},
			{`(~[a-z])(''')`, ByGroups(LiteralStringOther, LiteralStringHeredoc), Push("triapos-end", "triapos-intp")},
			{`(~[A-Z])(''')`, ByGroups(LiteralStringOther, LiteralStringHeredoc), Push("triapos-end", "triapos-no-intp")},
			{`~[a-z]\{`, LiteralStringOther, Push("cb-intp")},
			{`~[A-Z]\{`, LiteralStringOther, Push("cb-no-intp")},
			{`~[a-z]\[`, LiteralStringOther, Push("sb-intp")},
			{`~[A-Z]\[`, LiteralStringOther, Push("sb-no-intp")},
			{`~[a-z]\(`, LiteralStringOther, Push("pa-intp")},
			{`~[A-Z]\(`, LiteralStringOther, Push("pa-no-intp")},
			{`~[a-z]<`, LiteralStringOther, Push("ab-intp")},
			{`~[A-Z]<`, LiteralStringOther, Push("ab-no-intp")},
			{`~[a-z]/`, LiteralStringOther, Push("slas-intp")},
			{`~[A-Z]/`, LiteralStringOther, Push("slas-no-intp")},
			{`~[a-z]\|`, LiteralStringOther, Push("pipe-intp")},
			{`~[A-Z]\|`, LiteralStringOther, Push("pipe-no-intp")},
			{`~[a-z]"`, LiteralStringOther, Push("quot-intp")},
			{`~[A-Z]"`, LiteralStringOther, Push("quot-no-intp")},
			{`~[a-z]'`, LiteralStringOther, Push("apos-intp")},
			{`~[A-Z]'`, LiteralStringOther, Push("apos-no-intp")},
		},
		"triquot-end": {
			{`[a-zA-Z]+`, LiteralStringOther, Pop(1)},
			Default(Pop(1)),
		},
		"triquot-intp": {
			{`^\s*"""`, LiteralStringHeredoc, Pop(1)},
			Include("heredoc_interpol"),
		},
		"triquot-no-intp": {
			{`^\s*"""`, LiteralStringHeredoc, Pop(1)},
			Include("heredoc_no_interpol"),
		},
		"triapos-end": {
			{`[a-zA-Z]+`, LiteralStringOther, Pop(1)},
			Default(Pop(1)),
		},
		"triapos-intp": {
			{`^\s*'''`, LiteralStringHeredoc, Pop(1)},
			Include("heredoc_interpol"),
		},
		"triapos-no-intp": {
			{`^\s*'''`, LiteralStringHeredoc, Pop(1)},
			Include("heredoc_no_interpol"),
		},
		"cb-intp": {
			{`[^#\}\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`\}[a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"cb-no-intp": {
			{`[^\}\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`\}[a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
		"sb-intp": {
			{`[^#\]\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`\][a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"sb-no-intp": {
			{`[^\]\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`\][a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
		"pa-intp": {
			{`[^#\)\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`\)[a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"pa-no-intp": {
			{`[^\)\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`\)[a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
		"ab-intp": {
			{`[^#>\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`>[a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"ab-no-intp": {
			{`[^>\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`>[a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
		"slas-intp": {
			{`[^#/\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`/[a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"slas-no-intp": {
			{`[^/\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`/[a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
		"pipe-intp": {
			{`[^#\|\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`\|[a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"pipe-no-intp": {
			{`[^\|\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`\|[a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
		"quot-intp": {
			{`[^#"\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`"[a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"quot-no-intp": {
			{`[^"\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`"[a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
		"apos-intp": {
			{`[^#'\\]+`, LiteralStringOther, nil},
			Include("escapes"),
			{`\\.`, LiteralStringOther, nil},
			{`'[a-zA-Z]*`, LiteralStringOther, Pop(1)},
			Include("interpol"),
		},
		"apos-no-intp": {
			{`[^'\\]+`, LiteralStringOther, nil},
			{`\\.`, LiteralStringOther, nil},
			{`'[a-zA-Z]*`, LiteralStringOther, Pop(1)},
		},
	},
))
