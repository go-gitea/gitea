package editorconfig

import (
	"os"

	"gopkg.in/ini.v1"
)

// SimpleParser implements the Parser interface but without doing any caching.
type SimpleParser struct{}

// ParseIni calls go-ini's Load on the file.
func (parser *SimpleParser) ParseIni(filename string) (*Editorconfig, error) {
	fp, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer fp.Close()

	iniFile, err := ini.Load(fp)
	if err != nil {
		return nil, err
	}

	return newEditorconfig(iniFile)
}

// FnmatchCase calls the module's FnmatchCase.
func (parser *SimpleParser) FnmatchCase(selector string, filename string) (bool, error) {
	return FnmatchCase(selector, filename)
}
