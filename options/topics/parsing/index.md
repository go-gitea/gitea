---
aliases: parser-generator, ast, lexer, tokenizer, grammar, lexical-analysis, parse, bnf, backus-naur-form, lalr
display_name: Parsing
related: language, yacc, antlr, semantic-analysis, symbol-table, lookahead, ll, lr-parser, generated-parser
short_description: Parsing is the process of analyzing a string of symbols conforming to the rules of a formal grammar.
topic: parsing
wikipedia_url: https://en.wikipedia.org/wiki/Parsing#Computer_languages
---
A grammar describes the syntax of a programming language, and might be defined in Backus-Naur form (BNF). A lexer performs lexical analysis, turning text into tokens. A parser takes tokens and builds a data structure like an abstract syntax tree (AST). The parser is concerned with context: does the sequence of tokens fit the grammar? A compiler is a combined lexer and parser, built for a specific grammar.
