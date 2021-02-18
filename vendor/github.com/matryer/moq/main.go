package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/matryer/moq/pkg/moq"
)

type userFlags struct {
	outFile string
	pkgName string
	args    []string
}

func main() {
	var flags userFlags
	flag.StringVar(&flags.outFile, "out", "", "output file (default stdout)")
	flag.StringVar(&flags.pkgName, "pkg", "", "package name (default will infer)")

	flag.Usage = func() {
		fmt.Println(`moq [flags] destination interface [interface2 [interface3 [...]]]`)
		flag.PrintDefaults()
		fmt.Println(`Specifying an alias for the mock is also supported with the format 'interface:alias'`)
		fmt.Println(`Ex: moq -pkg different . MyInterface:MyMock`)
	}

	flag.Parse()
	flags.args = flag.Args()

	if err := run(flags); err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}
}

func run(flags userFlags) error {
	if len(flags.args) < 2 {
		return errors.New("not enough arguments")
	}

	var buf bytes.Buffer
	var out io.Writer = os.Stdout
	if flags.outFile != "" {
		out = &buf
	}

	destination := flags.args[0]
	args := flags.args[1:]
	m, err := moq.New(destination, flags.pkgName)
	if err != nil {
		return err
	}

	if err = m.Mock(out, args...); err != nil {
		return err
	}

	if flags.outFile == "" {
		return nil
	}

	// create the file
	err = os.MkdirAll(filepath.Dir(flags.outFile), 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(flags.outFile, buf.Bytes(), 0644)
}
