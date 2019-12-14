package org

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Table struct {
	Rows        []Row
	ColumnInfos []ColumnInfo
}

type Row struct {
	Columns   []Column
	IsSpecial bool
}

type Column struct {
	Children []Node
	*ColumnInfo
}

type ColumnInfo struct {
	Align string
	Len   int
}

var tableSeparatorRegexp = regexp.MustCompile(`^(\s*)(\|[+-|]*)\s*$`)
var tableRowRegexp = regexp.MustCompile(`^(\s*)(\|.*)`)

var columnAlignRegexp = regexp.MustCompile(`^<(l|c|r)>$`)

func lexTable(line string) (token, bool) {
	if m := tableSeparatorRegexp.FindStringSubmatch(line); m != nil {
		return token{"tableSeparator", len(m[1]), m[2], m}, true
	} else if m := tableRowRegexp.FindStringSubmatch(line); m != nil {
		return token{"tableRow", len(m[1]), m[2], m}, true
	}
	return nilToken, false
}

func (d *Document) parseTable(i int, parentStop stopFn) (int, Node) {
	rawRows, start := [][]string{}, i
	for ; !parentStop(d, i); i++ {
		if t := d.tokens[i]; t.kind == "tableRow" {
			rawRow := strings.FieldsFunc(d.tokens[i].content, func(r rune) bool { return r == '|' })
			for i := range rawRow {
				rawRow[i] = strings.TrimSpace(rawRow[i])
			}
			rawRows = append(rawRows, rawRow)
		} else if t.kind == "tableSeparator" {
			rawRows = append(rawRows, nil)
		} else {
			break
		}
	}

	table := Table{nil, getColumnInfos(rawRows)}
	for _, rawColumns := range rawRows {
		row := Row{nil, isSpecialRow(rawColumns)}
		if len(rawColumns) != 0 {
			for i := range table.ColumnInfos {
				column := Column{nil, &table.ColumnInfos[i]}
				if i < len(rawColumns) {
					column.Children = d.parseInline(rawColumns[i])
				}
				row.Columns = append(row.Columns, column)
			}
		}
		table.Rows = append(table.Rows, row)
	}
	return i - start, table
}

func getColumnInfos(rows [][]string) []ColumnInfo {
	columnCount := 0
	for _, columns := range rows {
		if n := len(columns); n > columnCount {
			columnCount = n
		}
	}

	columnInfos := make([]ColumnInfo, columnCount)
	for i := 0; i < columnCount; i++ {
		countNumeric, countNonNumeric := 0, 0
		for _, columns := range rows {
			if i >= len(columns) {
				continue
			}

			if n := utf8.RuneCountInString(columns[i]); n > columnInfos[i].Len {
				columnInfos[i].Len = n
			}

			if m := columnAlignRegexp.FindStringSubmatch(columns[i]); m != nil && isSpecialRow(columns) {
				switch m[1] {
				case "l":
					columnInfos[i].Align = "left"
				case "c":
					columnInfos[i].Align = "center"
				case "r":
					columnInfos[i].Align = "right"
				}
			} else if _, err := strconv.ParseFloat(columns[i], 32); err == nil {
				countNumeric++
			} else if strings.TrimSpace(columns[i]) != "" {
				countNonNumeric++
			}
		}

		if columnInfos[i].Align == "" && countNumeric >= countNonNumeric {
			columnInfos[i].Align = "right"
		}
	}
	return columnInfos
}

func isSpecialRow(rawColumns []string) bool {
	isAlignRow := true
	for _, rawColumn := range rawColumns {
		if !columnAlignRegexp.MatchString(rawColumn) && rawColumn != "" {
			isAlignRow = false
		}
	}
	return isAlignRow
}

func (n Table) String() string { return orgWriter.WriteNodesAsString(n) }
