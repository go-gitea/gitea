package generate

import "github.com/go-swagger/go-swagger/generator"

type Cli struct {
	// generate a cli includes all client code
	Client
}

func (c Cli) apply(opts *generator.GenOpts) {
	c.Client.apply(opts)
	opts.IncludeCLi = true
	opts.CliPackage = "cli" // hardcoded for now, can be exposed via cmd opt later
}

func (c *Cli) generate(opts *generator.GenOpts) error {
	return c.Client.generate(opts)
}

// Execute runs this command
func (c *Cli) Execute(args []string) error {
	return createSwagger(c)
}
