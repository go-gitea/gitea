// +build prebuild

package main

// prebuild.go generates sort implementations for
// various slice types and combination slice+reflect.Value types.
//
// The combination slice+reflect.Value types are used
// during canonical encode, and the others are used during fast-path
// encoding of map keys.

import (
	"bytes"
	"go/format"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

// genInternalSortableTypes returns the types
// that are used for fast-path canonical's encoding of maps.
//
// For now, we only support the highest sizes for
// int64, uint64, float64, bool, string, bytes.
func genInternalSortableTypes() []string {
	return []string{
		"string",
		// "float32",
		"float64",
		// "uint",
		// "uint8",
		// "uint16",
		// "uint32",
		"uint64",
		"uintptr",
		// "int",
		// "int8",
		// "int16",
		// "int32",
		"int64",
		"bool",
		"time",
		"bytes",
	}
}

// genInternalSortablePlusTypes returns the types
// that are used for reflection-based canonical's encoding of maps.
//
// For now, we only support the highest sizes for
// int64, uint64, float64, bool, string, bytes.
func genInternalSortablePlusTypes() []string {
	return []string{
		"string",
		"float64",
		"uint64",
		"uintptr",
		"int64",
		"bool",
		"time",
		"bytes",
	}
}

func genTypeForShortName(s string) string {
	switch s {
	case "time":
		return "time.Time"
	case "bytes":
		return "[]byte"
	}
	return s
}

func genArgs(args ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(args)/2)
	for i := 0; i < len(args); {
		m[args[i].(string)] = args[i+1]
		i += 2
	}
	return m
}

func genEndsWith(s0 string, sn ...string) bool {
	for _, s := range sn {
		if strings.HasSuffix(s0, s) {
			return true
		}
	}
	return false
}

func chkerr(err error) {
	if err != nil {
		panic(err)
	}
}

func run(fnameIn, fnameOut string) {
	var err error

	funcs := make(template.FuncMap)
	funcs["sortables"] = genInternalSortableTypes
	funcs["sortablesplus"] = genInternalSortablePlusTypes
	funcs["tshort"] = genTypeForShortName
	funcs["endswith"] = genEndsWith
	funcs["args"] = genArgs

	t := template.New("").Funcs(funcs)
	fin, err := os.Open(fnameIn)
	chkerr(err)
	defer fin.Close()
	fout, err := os.Create(fnameOut)
	chkerr(err)
	defer fout.Close()
	tmplstr, err := ioutil.ReadAll(fin)
	chkerr(err)
	t, err = t.Parse(string(tmplstr))
	chkerr(err)
	var out bytes.Buffer
	err = t.Execute(&out, 0)
	chkerr(err)
	bout, err := format.Source(out.Bytes())
	if err != nil {
		fout.Write(out.Bytes()) // write out if error, so we can still see.
	}
	chkerr(err)
	// write out if error, as much as possible, so we can still see.
	_, err = fout.Write(bout)
	chkerr(err)
}

func main() {
	run("sort-slice.go.tmpl", "sort-slice.generated.go")
}
