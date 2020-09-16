package codescan

import (
	"go/ast"
	"strconv"
	"strings"
)

func getEnumBasicLitValue(basicLit *ast.BasicLit) interface{} {
	switch basicLit.Kind.String() {
	case "INT":
		if result, err := strconv.ParseInt(basicLit.Value, 10, 64); err == nil {
			return result
		}
	case "FLOAT":
		if result, err := strconv.ParseFloat(basicLit.Value, 64); err == nil {
			return result
		}
	default:
		return strings.Trim(basicLit.Value, "\"")
	}
	return nil
}
