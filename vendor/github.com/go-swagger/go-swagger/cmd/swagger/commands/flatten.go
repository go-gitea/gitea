package commands

import (
	"errors"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-swagger/go-swagger/cmd/swagger/commands/generate"
	flags "github.com/jessevdk/go-flags"
)

// FlattenSpec is a command that flattens a swagger document
// which will expand the remote references in a spec and move inline schemas to definitions
// after flattening there are no complex inlined anymore
type FlattenSpec struct {
	Compact bool           `long:"compact" description:"applies to JSON formatted specs. When present, doesn't prettify the json"`
	Output  flags.Filename `long:"output" short:"o" description:"the file to write to"`
	Format  string         `long:"format" description:"the format for the spec document" default:"json" choice:"yaml" choice:"json"`
	generate.FlattenCmdOptions
}

// Execute flattens the spec
func (c *FlattenSpec) Execute(args []string) error {
	if len(args) != 1 {
		return errors.New("flatten command requires the single swagger document url to be specified")
	}

	swaggerDoc := args[0]
	specDoc, err := loads.Spec(swaggerDoc)
	if err != nil {
		return err
	}

	flattenOpts := c.FlattenCmdOptions.SetFlattenOptions(&analysis.FlattenOpts{
		// defaults
		Minimal:      true,
		Verbose:      true,
		Expand:       false,
		RemoveUnused: false,
	})
	flattenOpts.BasePath = specDoc.SpecFilePath()
	flattenOpts.Spec = analysis.New(specDoc.Spec())
	if err := analysis.Flatten(*flattenOpts); err != nil {
		return err
	}

	return writeToFile(specDoc.Spec(), !c.Compact, c.Format, string(c.Output))
}
