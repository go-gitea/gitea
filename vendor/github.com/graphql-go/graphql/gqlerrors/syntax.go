package gqlerrors

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/location"
	"github.com/graphql-go/graphql/language/source"
)

func NewSyntaxError(s *source.Source, position int, description string) *Error {
	l := location.GetLocation(s, position)
	return NewError(
		fmt.Sprintf("Syntax Error %s (%d:%d) %s\n\n%s", s.Name, l.Line, l.Column, description, highlightSourceAtLocation(s, l)),
		[]ast.Node{},
		"",
		s,
		[]int{position},
		nil,
	)
}

// printCharCode here is slightly different from lexer.printCharCode()
func printCharCode(code rune) string {
	// print as ASCII for printable range
	if code >= 0x0020 {
		return fmt.Sprintf(`%c`, code)
	}
	// Otherwise print the escaped form. e.g. `"\\u0007"`
	return fmt.Sprintf(`\u%04X`, code)
}
func printLine(str string) string {
	strSlice := []string{}
	for _, runeValue := range str {
		strSlice = append(strSlice, printCharCode(runeValue))
	}
	return fmt.Sprintf(`%s`, strings.Join(strSlice, ""))
}
func highlightSourceAtLocation(s *source.Source, l location.SourceLocation) string {
	line := l.Line
	prevLineNum := fmt.Sprintf("%d", (line - 1))
	lineNum := fmt.Sprintf("%d", line)
	nextLineNum := fmt.Sprintf("%d", (line + 1))
	padLen := len(nextLineNum)
	lines := regexp.MustCompile("\r\n|[\n\r]").Split(string(s.Body), -1)
	var highlight string
	if line >= 2 {
		highlight += fmt.Sprintf("%s: %s\n", lpad(padLen, prevLineNum), printLine(lines[line-2]))
	}
	highlight += fmt.Sprintf("%s: %s\n", lpad(padLen, lineNum), printLine(lines[line-1]))
	for i := 1; i < (2 + padLen + l.Column); i++ {
		highlight += " "
	}
	highlight += "^\n"
	if line < len(lines) {
		highlight += fmt.Sprintf("%s: %s\n", lpad(padLen, nextLineNum), printLine(lines[line]))
	}
	return highlight
}

func lpad(l int, s string) string {
	var r string
	for i := 1; i < (l - len(s) + 1); i++ {
		r += " "
	}
	return r + s
}
