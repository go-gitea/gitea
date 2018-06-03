package excelize

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// parseFormatTableSet provides function to parse the format settings of the
// table with default value.
func parseFormatTableSet(formatSet string) (*formatTable, error) {
	format := formatTable{
		TableStyle:     "",
		ShowRowStripes: true,
	}
	err := json.Unmarshal([]byte(formatSet), &format)
	return &format, err
}

// AddTable provides the method to add table in a worksheet by given worksheet
// name, coordinate area and format set. For example, create a table of A1:D5
// on Sheet1:
//
//    xlsx.AddTable("Sheet1", "A1", "D5", ``)
//
// Create a table of F2:H6 on Sheet2 with format set:
//
//    xlsx.AddTable("Sheet2", "F2", "H6", `{"table_name":"table","table_style":"TableStyleMedium2", "show_first_column":true,"show_last_column":true,"show_row_stripes":false,"show_column_stripes":true}`)
//
// Note that the table at least two lines include string type header. Multiple
// tables coordinate areas can't have an intersection.
//
// table_name: The name of the table, in the same worksheet name of the table should be unique
//
// table_style: The built-in table style names
//
//    TableStyleLight1 - TableStyleLight21
//    TableStyleMedium1 - TableStyleMedium28
//    TableStyleDark1 - TableStyleDark11
//
func (f *File) AddTable(sheet, hcell, vcell, format string) error {
	formatSet, err := parseFormatTableSet(format)
	if err != nil {
		return err
	}
	hcell = strings.ToUpper(hcell)
	vcell = strings.ToUpper(vcell)
	// Coordinate conversion, convert C1:B3 to 2,0,1,2.
	hcol := string(strings.Map(letterOnlyMapF, hcell))
	hrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, hcell))
	hyAxis := hrow - 1
	hxAxis := TitleToNumber(hcol)

	vcol := string(strings.Map(letterOnlyMapF, vcell))
	vrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, vcell))
	vyAxis := vrow - 1
	vxAxis := TitleToNumber(vcol)
	if vxAxis < hxAxis {
		vxAxis, hxAxis = hxAxis, vxAxis
	}
	if vyAxis < hyAxis {
		vyAxis, hyAxis = hyAxis, vyAxis
	}
	tableID := f.countTables() + 1
	sheetRelationshipsTableXML := "../tables/table" + strconv.Itoa(tableID) + ".xml"
	tableXML := strings.Replace(sheetRelationshipsTableXML, "..", "xl", -1)
	// Add first table for given sheet.
	rID := f.addSheetRelationships(sheet, SourceRelationshipTable, sheetRelationshipsTableXML, "")
	f.addSheetTable(sheet, rID)
	f.addTable(sheet, tableXML, hxAxis, hyAxis, vxAxis, vyAxis, tableID, formatSet)
	f.addContentTypePart(tableID, "table")
	return err
}

// countTables provides function to get table files count storage in the folder
// xl/tables.
func (f *File) countTables() int {
	count := 0
	for k := range f.XLSX {
		if strings.Contains(k, "xl/tables/table") {
			count++
		}
	}
	return count
}

// addSheetTable provides function to add tablePart element to
// xl/worksheets/sheet%d.xml by given worksheet name and relationship index.
func (f *File) addSheetTable(sheet string, rID int) {
	xlsx := f.workSheetReader(sheet)
	table := &xlsxTablePart{
		RID: "rId" + strconv.Itoa(rID),
	}
	if xlsx.TableParts == nil {
		xlsx.TableParts = &xlsxTableParts{}
	}
	xlsx.TableParts.Count++
	xlsx.TableParts.TableParts = append(xlsx.TableParts.TableParts, table)
}

// addTable provides function to add table by given worksheet name, coordinate
// area and format set.
func (f *File) addTable(sheet, tableXML string, hxAxis, hyAxis, vxAxis, vyAxis, i int, formatSet *formatTable) {
	// Correct the minimum number of rows, the table at least two lines.
	if hyAxis == vyAxis {
		vyAxis++
	}
	// Correct table reference coordinate area, such correct C1:B3 to B1:C3.
	ref := ToAlphaString(hxAxis) + strconv.Itoa(hyAxis+1) + ":" + ToAlphaString(vxAxis) + strconv.Itoa(vyAxis+1)
	tableColumn := []*xlsxTableColumn{}
	idx := 0
	for i := hxAxis; i <= vxAxis; i++ {
		idx++
		cell := ToAlphaString(i) + strconv.Itoa(hyAxis+1)
		name := f.GetCellValue(sheet, cell)
		if _, err := strconv.Atoi(name); err == nil {
			f.SetCellStr(sheet, cell, name)
		}
		if name == "" {
			name = "Column" + strconv.Itoa(idx)
			f.SetCellStr(sheet, cell, name)
		}
		tableColumn = append(tableColumn, &xlsxTableColumn{
			ID:   idx,
			Name: name,
		})
	}
	name := formatSet.TableName
	if name == "" {
		name = "Table" + strconv.Itoa(i)
	}
	t := xlsxTable{
		XMLNS:       NameSpaceSpreadSheet,
		ID:          i,
		Name:        name,
		DisplayName: name,
		Ref:         ref,
		AutoFilter: &xlsxAutoFilter{
			Ref: ref,
		},
		TableColumns: &xlsxTableColumns{
			Count:       idx,
			TableColumn: tableColumn,
		},
		TableStyleInfo: &xlsxTableStyleInfo{
			Name:              formatSet.TableStyle,
			ShowFirstColumn:   formatSet.ShowFirstColumn,
			ShowLastColumn:    formatSet.ShowLastColumn,
			ShowRowStripes:    formatSet.ShowRowStripes,
			ShowColumnStripes: formatSet.ShowColumnStripes,
		},
	}
	table, _ := xml.Marshal(t)
	f.saveFileList(tableXML, table)
}

// parseAutoFilterSet provides function to parse the settings of the auto
// filter.
func parseAutoFilterSet(formatSet string) (*formatAutoFilter, error) {
	format := formatAutoFilter{}
	err := json.Unmarshal([]byte(formatSet), &format)
	return &format, err
}

// AutoFilter provides the method to add auto filter in a worksheet by given
// worksheet name, coordinate area and settings. An autofilter in Excel is a
// way of filtering a 2D range of data based on some simple criteria. For
// example applying an autofilter to a cell range A1:D4 in the Sheet1:
//
//    err = xlsx.AutoFilter("Sheet1", "A1", "D4", "")
//
// Filter data in an autofilter:
//
//    err = xlsx.AutoFilter("Sheet1", "A1", "D4", `{"column":"B","expression":"x != blanks"}`)
//
// column defines the filter columns in a autofilter range based on simple
// criteria
//
// It isn't sufficient to just specify the filter condition. You must also
// hide any rows that don't match the filter condition. Rows are hidden using
// the SetRowVisible() method. Excelize can't filter rows automatically since
// this isn't part of the file format.
//
// Setting a filter criteria for a column:
//
// expression defines the conditions, the following operators are available
// for setting the filter criteria:
//
//    ==
//    !=
//    >
//    <
//    >=
//    <=
//    and
//    or
//
// An expression can comprise a single statement or two statements separated
// by the 'and' and 'or' operators. For example:
//
//    x <  2000
//    x >  2000
//    x == 2000
//    x >  2000 and x <  5000
//    x == 2000 or  x == 5000
//
// Filtering of blank or non-blank data can be achieved by using a value of
// Blanks or NonBlanks in the expression:
//
//    x == Blanks
//    x == NonBlanks
//
// Excel also allows some simple string matching operations:
//
//    x == b*      // begins with b
//    x != b*      // doesnt begin with b
//    x == *b      // ends with b
//    x != *b      // doesnt end with b
//    x == *b*     // contains b
//    x != *b*     // doesn't contains b
//
// You can also use '*' to match any character or number and '?' to match any
// single character or number. No other regular expression quantifier is
// supported by Excel's filters. Excel's regular expression characters can be
// escaped using '~'.
//
// The placeholder variable x in the above examples can be replaced by any
// simple string. The actual placeholder name is ignored internally so the
// following are all equivalent:
//
//    x     < 2000
//    col   < 2000
//    Price < 2000
//
func (f *File) AutoFilter(sheet, hcell, vcell, format string) error {
	formatSet, _ := parseAutoFilterSet(format)

	hcell = strings.ToUpper(hcell)
	vcell = strings.ToUpper(vcell)

	// Coordinate conversion, convert C1:B3 to 2,0,1,2.
	hcol := string(strings.Map(letterOnlyMapF, hcell))
	hrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, hcell))
	hyAxis := hrow - 1
	hxAxis := TitleToNumber(hcol)

	vcol := string(strings.Map(letterOnlyMapF, vcell))
	vrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, vcell))
	vyAxis := vrow - 1
	vxAxis := TitleToNumber(vcol)

	if vxAxis < hxAxis {
		vxAxis, hxAxis = hxAxis, vxAxis
	}

	if vyAxis < hyAxis {
		vyAxis, hyAxis = hyAxis, vyAxis
	}
	ref := ToAlphaString(hxAxis) + strconv.Itoa(hyAxis+1) + ":" + ToAlphaString(vxAxis) + strconv.Itoa(vyAxis+1)
	refRange := vxAxis - hxAxis
	return f.autoFilter(sheet, ref, refRange, hxAxis, formatSet)
}

// autoFilter provides function to extract the tokens from the filter
// expression. The tokens are mainly non-whitespace groups.
func (f *File) autoFilter(sheet, ref string, refRange, hxAxis int, formatSet *formatAutoFilter) error {
	xlsx := f.workSheetReader(sheet)
	if xlsx.SheetPr != nil {
		xlsx.SheetPr.FilterMode = true
	}
	xlsx.SheetPr = &xlsxSheetPr{FilterMode: true}
	filter := &xlsxAutoFilter{
		Ref: ref,
	}
	xlsx.AutoFilter = filter
	if formatSet.Column == "" || formatSet.Expression == "" {
		return nil
	}
	col := TitleToNumber(formatSet.Column)
	offset := col - hxAxis
	if offset < 0 || offset > refRange {
		return fmt.Errorf("Incorrect index of column '%s'", formatSet.Column)
	}
	filter.FilterColumn = &xlsxFilterColumn{
		ColID: offset,
	}
	re := regexp.MustCompile(`"(?:[^"]|"")*"|\S+`)
	token := re.FindAllString(formatSet.Expression, -1)
	if len(token) != 3 && len(token) != 7 {
		return fmt.Errorf("Incorrect number of tokens in criteria '%s'", formatSet.Expression)
	}
	expressions, tokens, err := f.parseFilterExpression(formatSet.Expression, token)
	if err != nil {
		return err
	}
	f.writeAutoFilter(filter, expressions, tokens)
	xlsx.AutoFilter = filter
	return nil
}

// writeAutoFilter provides function to check for single or double custom filters
// as default filters and handle them accordingly.
func (f *File) writeAutoFilter(filter *xlsxAutoFilter, exp []int, tokens []string) {
	if len(exp) == 1 && exp[0] == 2 {
		// Single equality.
		filters := []*xlsxFilter{}
		filters = append(filters, &xlsxFilter{Val: tokens[0]})
		filter.FilterColumn.Filters = &xlsxFilters{Filter: filters}
	} else if len(exp) == 3 && exp[0] == 2 && exp[1] == 1 && exp[2] == 2 {
		// Double equality with "or" operator.
		filters := []*xlsxFilter{}
		for _, v := range tokens {
			filters = append(filters, &xlsxFilter{Val: v})
		}
		filter.FilterColumn.Filters = &xlsxFilters{Filter: filters}
	} else {
		// Non default custom filter.
		expRel := map[int]int{0: 0, 1: 2}
		andRel := map[int]bool{0: true, 1: false}
		for k, v := range tokens {
			f.writeCustomFilter(filter, exp[expRel[k]], v)
			if k == 1 {
				filter.FilterColumn.CustomFilters.And = andRel[exp[k]]
			}
		}
	}
}

// writeCustomFilter provides function to write the <customFilter> element.
func (f *File) writeCustomFilter(filter *xlsxAutoFilter, operator int, val string) {
	operators := map[int]string{
		1:  "lessThan",
		2:  "equal",
		3:  "lessThanOrEqual",
		4:  "greaterThan",
		5:  "notEqual",
		6:  "greaterThanOrEqual",
		22: "equal",
	}
	customFilter := xlsxCustomFilter{
		Operator: operators[operator],
		Val:      val,
	}
	if filter.FilterColumn.CustomFilters != nil {
		filter.FilterColumn.CustomFilters.CustomFilter = append(filter.FilterColumn.CustomFilters.CustomFilter, &customFilter)
	} else {
		customFilters := []*xlsxCustomFilter{}
		customFilters = append(customFilters, &customFilter)
		filter.FilterColumn.CustomFilters = &xlsxCustomFilters{CustomFilter: customFilters}
	}
}

// parseFilterExpression provides function to converts the tokens of a possibly
// conditional expression into 1 or 2 sub expressions for further parsing.
//
// Examples:
//
//    ('x', '==', 2000) -> exp1
//    ('x', '>',  2000, 'and', 'x', '<', 5000) -> exp1 and exp2
//
func (f *File) parseFilterExpression(expression string, tokens []string) ([]int, []string, error) {
	expressions := []int{}
	t := []string{}
	if len(tokens) == 7 {
		// The number of tokens will be either 3 (for 1 expression) or 7 (for 2
		// expressions).
		conditional := 0
		c := tokens[3]
		re, _ := regexp.Match(`(or|\|\|)`, []byte(c))
		if re {
			conditional = 1
		}
		expression1, token1, err := f.parseFilterTokens(expression, tokens[0:3])
		if err != nil {
			return expressions, t, err
		}
		expression2, token2, err := f.parseFilterTokens(expression, tokens[4:7])
		if err != nil {
			return expressions, t, err
		}
		expressions = []int{expression1[0], conditional, expression2[0]}
		t = []string{token1, token2}
	} else {
		exp, token, err := f.parseFilterTokens(expression, tokens)
		if err != nil {
			return expressions, t, err
		}
		expressions = exp
		t = []string{token}
	}
	return expressions, t, nil
}

// parseFilterTokens provides function to parse the 3 tokens of a filter
// expression and return the operator and token.
func (f *File) parseFilterTokens(expression string, tokens []string) ([]int, string, error) {
	operators := map[string]int{
		"==": 2,
		"=":  2,
		"=~": 2,
		"eq": 2,
		"!=": 5,
		"!~": 5,
		"ne": 5,
		"<>": 5,
		"<":  1,
		"<=": 3,
		">":  4,
		">=": 6,
	}
	operator, ok := operators[strings.ToLower(tokens[1])]
	if !ok {
		// Convert the operator from a number to a descriptive string.
		return []int{}, "", fmt.Errorf("Unknown operator: %s", tokens[1])
	}
	token := tokens[2]
	// Special handling for Blanks/NonBlanks.
	re, _ := regexp.Match("blanks|nonblanks", []byte(strings.ToLower(token)))
	if re {
		// Only allow Equals or NotEqual in this context.
		if operator != 2 && operator != 5 {
			return []int{operator}, token, fmt.Errorf("The operator '%s' in expression '%s' is not valid in relation to Blanks/NonBlanks'", tokens[1], expression)
		}
		token = strings.ToLower(token)
		// The operator should always be 2 (=) to flag a "simple" equality in
		// the binary record. Therefore we convert <> to =.
		if token == "blanks" {
			if operator == 5 {
				token = " "
			}
		} else {
			if operator == 5 {
				operator = 2
				token = "blanks"
			} else {
				operator = 5
				token = " "
			}
		}
	}
	// if the string token contains an Excel match character then change the
	// operator type to indicate a non "simple" equality.
	re, _ = regexp.Match("[*?]", []byte(token))
	if operator == 2 && re {
		operator = 22
	}
	return []int{operator}, token, nil
}
