package generator

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

type templateData struct {
	Package string
	Name    string
	KeyType *goType
	ValType *goType
}

type goType struct {
	Modifiers  string
	ImportPath string
	ImportName string
	Name       string
}

func (t *goType) String() string {
	if t.ImportName != "" {
		return t.Modifiers + t.ImportName + "." + t.Name
	}

	return t.Modifiers + t.Name
}

func (t *goType) IsPtr() bool {
	return strings.HasPrefix(t.Modifiers, "*")
}

func (t *goType) IsSlice() bool {
	return strings.HasPrefix(t.Modifiers, "[]")
}

var partsRe = regexp.MustCompile(`^([\[\]\*]*)(.*?)(\.\w*)?$`)

func parseType(str string) (*goType, error) {
	parts := partsRe.FindStringSubmatch(str)
	if len(parts) != 4 {
		return nil, fmt.Errorf("type must be in the form []*github.com/import/path.Name")
	}

	t := &goType{
		Modifiers:  parts[1],
		ImportPath: parts[2],
		Name:       strings.TrimPrefix(parts[3], "."),
	}

	if t.Name == "" {
		t.Name = t.ImportPath
		t.ImportPath = ""
	}

	if t.ImportPath != "" {
		p, err := packages.Load(&packages.Config{Mode: packages.NeedName}, t.ImportPath)
		if err != nil {
			return nil, err
		}
		if len(p) != 1 {
			return nil, fmt.Errorf("not found")
		}

		t.ImportName = p[0].Name
	}

	return t, nil
}

func Generate(name string, keyType string, valueType string, wd string) error {
	data, err := getData(name, keyType, valueType, wd)
	if err != nil {
		return err
	}

	filename := strings.ToLower(data.Name) + "_gen.go"

	if err := writeTemplate(filepath.Join(wd, filename), data); err != nil {
		return err
	}

	return nil
}

func getData(name string, keyType string, valueType string, wd string) (templateData, error) {
	var data templateData

	genPkg := getPackage(wd)
	if genPkg == nil {
		return templateData{}, fmt.Errorf("unable to find package info for " + wd)
	}

	var err error
	data.Name = name
	data.Package = genPkg.Name
	data.KeyType, err = parseType(keyType)
	if err != nil {
		return templateData{}, fmt.Errorf("key type: %s", err.Error())
	}
	data.ValType, err = parseType(valueType)
	if err != nil {
		return templateData{}, fmt.Errorf("key type: %s", err.Error())
	}

	// if we are inside the same package as the type we don't need an import and can refer directly to the type
	if genPkg.PkgPath == data.ValType.ImportPath {
		data.ValType.ImportName = ""
		data.ValType.ImportPath = ""
	}
	if genPkg.PkgPath == data.KeyType.ImportPath {
		data.KeyType.ImportName = ""
		data.KeyType.ImportPath = ""
	}

	return data, nil
}

func getPackage(dir string) *packages.Package {
	p, _ := packages.Load(&packages.Config{
		Dir: dir,
	}, ".")

	if len(p) != 1 {
		return nil
	}

	return p[0]
}

func writeTemplate(filepath string, data templateData) error {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return errors.Wrap(err, "generating code")
	}

	src, err := imports.Process(filepath, buf.Bytes(), nil)
	if err != nil {
		return errors.Wrap(err, "unable to gofmt")
	}

	if err := ioutil.WriteFile(filepath, src, 0644); err != nil {
		return errors.Wrap(err, "writing output")
	}

	return nil
}

func lcFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}
