// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"regexp"
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/terror"
)

// Error instances.
var (
	ErrSyntax = terror.ClassParser.New(CodeSyntaxErr, "syntax error")
)

// Error codes.
const (
	CodeSyntaxErr terror.ErrCode = 1
)

var (
	specCodePattern = regexp.MustCompile(`\/\*!(M?[0-9]{5,6})?([^*]|\*+[^*/])*\*+\/`)
	specCodeStart   = regexp.MustCompile(`^\/\*!(M?[0-9]{5,6} )?[ \t]*`)
	specCodeEnd     = regexp.MustCompile(`[ \t]*\*\/$`)
)

func trimComment(txt string) string {
	txt = specCodeStart.ReplaceAllString(txt, "")
	return specCodeEnd.ReplaceAllString(txt, "")
}

// See: http://dev.mysql.com/doc/refman/5.7/en/comments.html
// Convert "/*!VersionNumber MySQL-specific-code */" to "MySQL-specific-code".
// TODO: Find a better way:
// 1. RegExpr is slow.
// 2. Handle nested comment.
func handleMySQLSpecificCode(sql string) string {
	if strings.Index(sql, "/*!") == -1 {
		// Fast way to check if text contains MySQL-specific code.
		return sql
	}
	// SQL text contains MySQL-specific code. We should convert it to normal SQL text.
	return specCodePattern.ReplaceAllStringFunc(sql, trimComment)
}

// Parse parses a query string to raw ast.StmtNode.
// If charset or collation is "", default charset and collation will be used.
func Parse(sql, charset, collation string) ([]ast.StmtNode, error) {
	if charset == "" {
		charset = mysql.DefaultCharset
	}
	if collation == "" {
		collation = mysql.DefaultCollationName
	}
	sql = handleMySQLSpecificCode(sql)
	l := NewLexer(sql)
	l.SetCharsetInfo(charset, collation)
	yyParse(l)
	if len(l.Errors()) != 0 {
		return nil, errors.Trace(l.Errors()[0])
	}
	return l.Stmts(), nil
}

// ParseOneStmt parses a query and returns an ast.StmtNode.
// The query must have one statement, otherwise ErrSyntax is returned.
func ParseOneStmt(sql, charset, collation string) (ast.StmtNode, error) {
	stmts, err := Parse(sql, charset, collation)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(stmts) != 1 {
		return nil, ErrSyntax
	}
	return stmts[0], nil
}
