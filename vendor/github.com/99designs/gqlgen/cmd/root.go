package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/99designs/gqlgen/graphql"
	"github.com/urfave/cli/v2"

	// Required since otherwise dep will prune away these unused packages before codegen has a chance to run
	_ "github.com/99designs/gqlgen/graphql/handler"
	_ "github.com/99designs/gqlgen/handler"
)

func Execute() {
	app := cli.NewApp()
	app.Name = "gqlgen"
	app.Usage = genCmd.Usage
	app.Description = "This is a library for quickly creating strictly typed graphql servers in golang. See https://gqlgen.com/ for a getting started guide."
	app.HideVersion = true
	app.Flags = genCmd.Flags
	app.Version = graphql.Version
	app.Before = func(context *cli.Context) error {
		if context.Bool("verbose") {
			log.SetFlags(0)
		} else {
			log.SetOutput(ioutil.Discard)
		}
		return nil
	}

	app.Action = genCmd.Action
	app.Commands = []*cli.Command{
		genCmd,
		initCmd,
		versionCmd,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}
