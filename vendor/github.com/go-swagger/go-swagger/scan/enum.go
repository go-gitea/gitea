// +build !go1.11

package scan

import (
	"go/ast"
	"strconv"
	"strings"
	"unicode"
)

func upperSnakeCase(s string) string {
	in := []rune(s)
	isLower := func(idx int) bool {
		return idx >= 0 && idx < len(in) && unicode.IsLower(in[idx])
	}

	out := make([]rune, 0, len(in)+len(in)/2)

	for i, r := range in {
		if unicode.IsUpper(r) {
			r = unicode.ToLower(r)
			if i > 0 && in[i-1] != '_' && (isLower(i-1) || isLower(i+1)) {
				out = append(out, '_')
			}
		}
		out = append(out, r)
	}

	return strings.ToUpper(string(out))
}

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

func getEnumValues(file *ast.File, typeName string) (list []interface{}) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)

		if !ok {
			continue
		}

		if genDecl.Tok.String() == "const" {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					switch valueSpec.Type.(type) {
					case *ast.Ident:
						if valueSpec.Type.(*ast.Ident).Name == typeName {
							if basicLit, ok := valueSpec.Values[0].(*ast.BasicLit); ok {
								list = append(list, getEnumBasicLitValue(basicLit))
							}
						}
					default:
						var name = valueSpec.Names[0].Name
						if strings.HasPrefix(name, upperSnakeCase(typeName)) {
							var values = strings.SplitN(name, "__", 2)
							if len(values) == 2 {
								list = append(list, values[1])
							}
						}
					}

				}

			}
		}
	}
	return
}
