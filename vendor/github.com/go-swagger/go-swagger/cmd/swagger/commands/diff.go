package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"errors"

	"github.com/go-openapi/loads"
	"github.com/go-swagger/go-swagger/cmd/swagger/commands/diff"
)

// JSONFormat for json
const JSONFormat = "json"

// DiffCommand is a command that generates the diff of two swagger specs.
//
// There are no specific options for this expansion.
type DiffCommand struct {
	OnlyBreakingChanges bool   `long:"break" short:"b" description:"When present, only shows incompatible changes"`
	Format              string `long:"format" short:"f" description:"When present, writes output as json" default:"txt" choice:"txt" choice:"json"`
	IgnoreFile          string `long:"ignore" short:"i" description:"Exception file of diffs to ignore (copy output from json diff format)"  default:"none specified"`
	Destination         string `long:"dest" short:"d" description:"Output destination file or stdout" default:"stdout"`
	Args                struct {
		OldSpec string `positional-arg-name:"{old spec}"`
		NewSpec string `positional-arg-name:"{new spec}"`
	} `required:"2" positional-args:"specs" description:"Input specs to be diff-ed"`
}

// Execute diffs the two specs provided
func (c *DiffCommand) Execute(_ []string) error {
	if c.Args.OldSpec == "" || c.Args.NewSpec == "" {
		return errors.New(`missing arguments for diff command (use --help for more info)`)
	}

	c.printInfo()

	var (
		output io.WriteCloser
		err    error
	)
	if c.Destination != "stdout" {
		output, err = os.OpenFile(c.Destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Destination, err)
		}
		defer func() {
			_ = output.Close()
		}()
	} else {
		output = os.Stdout
	}

	diffs, err := c.getDiffs()
	if err != nil {
		return err
	}

	ignores, err := c.readIgnores()
	if err != nil {
		return err
	}

	diffs = diffs.FilterIgnores(ignores)
	if len(ignores) > 0 {
		log.Printf("Diff Report Ignored Items from IgnoreFile")
		for _, eachItem := range ignores {
			log.Printf("%s", eachItem.String())
		}
	}

	var (
		input io.Reader
		warn  error
	)
	if c.Format != JSONFormat && c.OnlyBreakingChanges {
		input, err, warn = diffs.ReportCompatibility()
	} else {
		input, err, warn = diffs.ReportAllDiffs(c.Format == JSONFormat)
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(output, input)
	if err != nil {
		return err
	}
	return warn
}

func (c *DiffCommand) readIgnores() (diff.SpecDifferences, error) {
	ignoreFile := c.IgnoreFile
	ignoreDiffs := diff.SpecDifferences{}

	if ignoreFile == "none specified" || ignoreFile == "" {
		return ignoreDiffs, nil
	}
	// Open our jsonFile
	jsonFile, err := os.Open(ignoreFile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ignoreFile, err)
	}
	defer func() {
		_ = jsonFile.Close()
	}()
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ignoreFile, err)
	}
	err = json.Unmarshal(byteValue, &ignoreDiffs)
	if err != nil {
		return nil, err
	}
	return ignoreDiffs, nil
}

func (c *DiffCommand) getDiffs() (diff.SpecDifferences, error) {
	oldSpecPath, newSpecPath := c.Args.OldSpec, c.Args.NewSpec
	swaggerDoc1 := oldSpecPath
	specDoc1, err := loads.Spec(swaggerDoc1)
	if err != nil {
		return nil, err
	}

	swaggerDoc2 := newSpecPath
	specDoc2, err := loads.Spec(swaggerDoc2)
	if err != nil {
		return nil, err
	}

	return diff.Compare(specDoc1.Spec(), specDoc2.Spec())
}

func (c *DiffCommand) printInfo() {
	log.Println("Run Config:")
	log.Printf("Spec1: %s", c.Args.OldSpec)
	log.Printf("Spec2: %s", c.Args.NewSpec)
	log.Printf("ReportOnlyBreakingChanges (-c) :%v", c.OnlyBreakingChanges)
	log.Printf("OutputFormat (-f) :%s", c.Format)
	log.Printf("IgnoreFile (-i) :%s", c.IgnoreFile)
	log.Printf("Diff Report Destination (-d) :%s", c.Destination)
}
