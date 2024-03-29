package merlin

import (
	"bytes"
	"encoding/base64"
	"errors"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"
)

type FrontMatter struct {
	License string `yaml:"license"`
}

func CheckLicense(content string) error {
	license, err := parseLicense(content)
	if license == "" {
		return err
	}

	if len(validLicense) == 0 {
		initConfig()
	}

	if !validLicense.Has(license) {
		return errors.New("invalid license")
	}

	return nil
}

func parseLicense(content string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return "", err
	}

	md := goldmark.New(
		goldmark.WithExtensions(&frontmatter.Extender{}),
	)

	ctx := parser.NewContext()
	var buf bytes.Buffer
	if err = md.Convert(b, &buf, parser.WithContext(ctx)); err != nil {
		return "", err
	}

	data := frontmatter.Get(ctx)
	if data == nil {
		return "", nil
	}

	var meta FrontMatter
	if err = data.Decode(&meta); err != nil {
		return "", err
	}

	return meta.License, nil
}
