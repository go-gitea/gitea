package r

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// Rust lexer.
var Rust = internal.Register(MustNewLexer(
	&Config{
		Name:      "Rust",
		Aliases:   []string{"rust"},
		Filenames: []string{"*.rs", "*.rs.in"},
		MimeTypes: []string{"text/rust"},
		EnsureNL:  true,
	},
	Rules{
		"root": {
			{`#![^[\r\n].*$`, CommentPreproc, nil},
			Default(Push("base")),
		},
		"base": {
			{`\n`, TextWhitespace, nil},
			{`\s+`, TextWhitespace, nil},
			{`//!.*?\n`, LiteralStringDoc, nil},
			{`///(\n|[^/].*?\n)`, LiteralStringDoc, nil},
			{`//(.*?)\n`, CommentSingle, nil},
			{`/\*\*(\n|[^/*])`, LiteralStringDoc, Push("doccomment")},
			{`/\*!`, LiteralStringDoc, Push("doccomment")},
			{`/\*`, CommentMultiline, Push("comment")},
			{`r#*"(?:\\.|[^\\;])*"#*`, LiteralString, nil},
			{`"(?:\\.|[^\\"])*"`, LiteralString, nil},
			{`\$([a-zA-Z_]\w*|\(,?|\),?|,?)`, CommentPreproc, nil},
			{Words(``, `\b`, `as`, `async`, `await`, `const`, `crate`, `else`, `extern`, `for`, `if`, `impl`, `in`, `loop`, `match`, `move`, `mut`, `pub`, `ref`, `return`, `static`, `super`, `trait`, `unsafe`, `use`, `where`, `while`), Keyword, nil},
			{Words(``, `\b`, `abstract`, `become`, `box`, `do`, `final`, `macro`, `override`, `priv`, `try`, `typeof`, `unsized`, `virtual`, `yield`), KeywordReserved, nil},
			{`(true|false)\b`, KeywordConstant, nil},
			{`mod\b`, Keyword, Push("modname")},
			{`let\b`, KeywordDeclaration, nil},
			{`fn\b`, Keyword, Push("funcname")},
			{`(struct|enum|type|union)\b`, Keyword, Push("typename")},
			{`(default)(\s+)(type|fn)\b`, ByGroups(Keyword, Text, Keyword), nil},
			{Words(``, `\b`, `u8`, `u16`, `u32`, `u64`, `u128`, `i8`, `i16`, `i32`, `i64`, `i128`, `usize`, `isize`, `f32`, `f64`, `str`, `bool`), KeywordType, nil},
			{`self\b`, NameBuiltinPseudo, nil},
			{Words(``, `\b`, `Copy`, `Send`, `Sized`, `Sync`, `Drop`, `Fn`, `FnMut`, `FnOnce`, `Box`, `ToOwned`, `Clone`, `PartialEq`, `PartialOrd`, `Eq`, `Ord`, `AsRef`, `AsMut`, `Into`, `From`, `Default`, `Iterator`, `Extend`, `IntoIterator`, `DoubleEndedIterator`, `ExactSizeIterator`, `Option`, `Some`, `None`, `Result`, `Ok`, `Err`, `SliceConcatExt`, `String`, `ToString`, `Vec`), NameBuiltin, nil},
			{`::\b`, Text, nil},
			{`(?::|->)`, Text, Push("typename")},
			{`(break|continue)(\s*)(\'[A-Za-z_]\w*)?`, ByGroups(Keyword, TextWhitespace, NameLabel), nil},
			{`'(\\['"\\nrt]|\\x[0-7][0-9a-fA-F]|\\0|\\u\{[0-9a-fA-F]{1,6}\}|.)'`, LiteralStringChar, nil},
			{`b'(\\['"\\nrt]|\\x[0-9a-fA-F]{2}|\\0|\\u\{[0-9a-fA-F]{1,6}\}|.)'`, LiteralStringChar, nil},
			{`0b[01_]+`, LiteralNumberBin, Push("number_lit")},
			{`0o[0-7_]+`, LiteralNumberOct, Push("number_lit")},
			{`0[xX][0-9a-fA-F_]+`, LiteralNumberHex, Push("number_lit")},
			{`[0-9][0-9_]*(\.[0-9_]+[eE][+\-]?[0-9_]+|\.[0-9_]*(?!\.)|[eE][+\-]?[0-9_]+)`, LiteralNumberFloat, Push("number_lit")},
			{`[0-9][0-9_]*`, LiteralNumberInteger, Push("number_lit")},
			{`b"`, LiteralString, Push("bytestring")},
			{`b?r(#*)".*?"\1`, LiteralString, nil},
			{`'static`, NameBuiltin, nil},
			{`'[a-zA-Z_]\w*`, NameAttribute, nil},
			{`[{}()\[\],.;]`, Punctuation, nil},
			{`[+\-*/%&|<>^!~@=:?]`, Operator, nil},
			{`(r#)?[a-zA-Z_]\w*`, Name, nil},
			{`#!?\[`, CommentPreproc, Push("attribute[")},
			{`([A-Za-z_]\w*)(!)(\s*)([A-Za-z_]\w*)?(\s*)(\{)`, ByGroups(CommentPreproc, Punctuation, TextWhitespace, Name, TextWhitespace, Punctuation), Push("macro{")},
			{`([A-Za-z_]\w*)(!)(\s*)([A-Za-z_]\w*)?(\()`, ByGroups(CommentPreproc, Punctuation, TextWhitespace, Name, Punctuation), Push("macro(")},
		},
		"comment": {
			{`[^*/]+`, CommentMultiline, nil},
			{`/\*`, CommentMultiline, Push()},
			{`\*/`, CommentMultiline, Pop(1)},
			{`[*/]`, CommentMultiline, nil},
		},
		"doccomment": {
			{`[^*/]+`, LiteralStringDoc, nil},
			{`/\*`, LiteralStringDoc, Push()},
			{`\*/`, LiteralStringDoc, Pop(1)},
			{`[*/]`, LiteralStringDoc, nil},
		},
		"modname": {
			{`\s+`, Text, nil},
			{`[a-zA-Z_]\w*`, NameNamespace, Pop(1)},
			Default(Pop(1)),
		},
		"funcname": {
			{`\s+`, Text, nil},
			{`[a-zA-Z_]\w*`, NameFunction, Pop(1)},
			Default(Pop(1)),
		},
		"typename": {
			{`\s+`, Text, nil},
			{`&`, KeywordPseudo, nil},
			{Words(``, `\b`, `Copy`, `Send`, `Sized`, `Sync`, `Drop`, `Fn`, `FnMut`, `FnOnce`, `Box`, `ToOwned`, `Clone`, `PartialEq`, `PartialOrd`, `Eq`, `Ord`, `AsRef`, `AsMut`, `Into`, `From`, `Default`, `Iterator`, `Extend`, `IntoIterator`, `DoubleEndedIterator`, `ExactSizeIterator`, `Option`, `Some`, `None`, `Result`, `Ok`, `Err`, `SliceConcatExt`, `String`, `ToString`, `Vec`), NameBuiltin, nil},
			{Words(``, `\b`, `u8`, `u16`, `u32`, `u64`, `i8`, `i16`, `i32`, `i64`, `usize`, `isize`, `f32`, `f64`, `str`, `bool`), KeywordType, nil},
			{`[a-zA-Z_]\w*`, NameClass, Pop(1)},
			Default(Pop(1)),
		},
		"number_lit": {
			{`[ui](8|16|32|64|size)`, Keyword, Pop(1)},
			{`f(32|64)`, Keyword, Pop(1)},
			Default(Pop(1)),
		},
		"string": {
			{`"`, LiteralString, Pop(1)},
			{`\\['"\\nrt]|\\x[0-7][0-9a-fA-F]|\\0|\\u\{[0-9a-fA-F]{1,6}\}`, LiteralStringEscape, nil},
			{`[^\\"]+`, LiteralString, nil},
			{`\\`, LiteralString, nil},
		},
		"bytestring": {
			{`\\x[89a-fA-F][0-9a-fA-F]`, LiteralStringEscape, nil},
			Include("string"),
		},
		"macro{": {
			{`\{`, Operator, Push()},
			{`\}`, Operator, Pop(1)},
		},
		"macro(": {
			{`\(`, Operator, Push()},
			{`\)`, Operator, Pop(1)},
		},
		"attribute_common": {
			{`"`, LiteralString, Push("string")},
			{`\[`, CommentPreproc, Push("attribute[")},
			{`\(`, CommentPreproc, Push("attribute(")},
		},
		"attribute[": {
			Include("attribute_common"),
			{`\];?`, CommentPreproc, Pop(1)},
			{`[^"\]]+`, CommentPreproc, nil},
		},
		"attribute(": {
			Include("attribute_common"),
			{`\);?`, CommentPreproc, Pop(1)},
			{`[^")]+`, CommentPreproc, nil},
		},
	},
))
