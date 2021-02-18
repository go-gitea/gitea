package cmd

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/internal/code"
	"github.com/99designs/gqlgen/plugin/servergen"
	"github.com/urfave/cli/v2"
)

var configTemplate = template.Must(template.New("name").Parse(
	`# Where are all the schema files located? globs are supported eg  src/**/*.graphqls
schema:
  - graph/*.graphqls

# Where should the generated server code go?
exec:
  filename: graph/generated/generated.go
  package: generated

# Uncomment to enable federation
# federation:
#   filename: graph/generated/federation.go
#   package: generated

# Where should any generated models go?
model:
  filename: graph/model/models_gen.go
  package: model

# Where should the resolver implementations go?
resolver:
  layout: follow-schema
  dir: graph
  package: graph

# Optional: turn on use ` + "`" + `gqlgen:"fieldName"` + "`" + ` tags in your models
# struct_tag: json

# Optional: turn on to use []Thing instead of []*Thing
# omit_slice_element_pointers: false

# Optional: set to speed up generation time by not performing a final validation pass.
# skip_validation: true

# gqlgen will search for any type names in the schema in these go packages
# if they match it will use them, otherwise it will generate them.
autobind:
  - "{{.}}/graph/model"

# This section declares type mapping between the GraphQL and go type systems
#
# The first line in each type will be used as defaults for resolver arguments and
# modelgen, the others will be allowed when binding to fields. Configure them to
# your liking
models:
  ID:
    model:
      - github.com/99designs/gqlgen/graphql.ID
      - github.com/99designs/gqlgen/graphql.Int
      - github.com/99designs/gqlgen/graphql.Int64
      - github.com/99designs/gqlgen/graphql.Int32
  Int:
    model:
      - github.com/99designs/gqlgen/graphql.Int
      - github.com/99designs/gqlgen/graphql.Int64
      - github.com/99designs/gqlgen/graphql.Int32
`))

var schemaDefault = `# GraphQL schema example
#
# https://gqlgen.com/getting-started/

type Todo {
  id: ID!
  text: String!
  done: Boolean!
  user: User!
}

type User {
  id: ID!
  name: String!
}

type Query {
  todos: [Todo!]!
}

input NewTodo {
  text: String!
  userId: String!
}

type Mutation {
  createTodo(input: NewTodo!): Todo!
}
`

var initCmd = &cli.Command{
	Name:  "init",
	Usage: "create a new gqlgen project",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "verbose, v", Usage: "show logs"},
		&cli.StringFlag{Name: "config, c", Usage: "the config filename"},
		&cli.StringFlag{Name: "server", Usage: "where to write the server stub to", Value: "server.go"},
		&cli.StringFlag{Name: "schema", Usage: "where to write the schema stub to", Value: "graph/schema.graphqls"},
	},
	Action: func(ctx *cli.Context) error {
		configFilename := ctx.String("config")
		serverFilename := ctx.String("server")

		pkgName := code.ImportPathForDir(".")
		if pkgName == "" {
			return fmt.Errorf("unable to determine import path for current directory, you probably need to run go mod init first")
		}

		if err := initSchema(ctx.String("schema")); err != nil {
			return err
		}
		if !configExists(configFilename) {
			if err := initConfig(configFilename, pkgName); err != nil {
				return err
			}
		}

		GenerateGraphServer(serverFilename)
		return nil
	},
}

func GenerateGraphServer(serverFilename string) {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	if err := api.Generate(cfg, api.AddPlugin(servergen.New(serverFilename))); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	fmt.Fprintf(os.Stdout, "Exec \"go run ./%s\" to start GraphQL server\n", serverFilename)
}

func configExists(configFilename string) bool {
	var cfg *config.Config

	if configFilename != "" {
		cfg, _ = config.LoadConfig(configFilename)
	} else {
		cfg, _ = config.LoadConfigFromDefaultLocations()
	}
	return cfg != nil
}

func initConfig(configFilename string, pkgName string) error {
	if configFilename == "" {
		configFilename = "gqlgen.yml"
	}

	if err := os.MkdirAll(filepath.Dir(configFilename), 0755); err != nil {
		return fmt.Errorf("unable to create config dir: " + err.Error())
	}

	var buf bytes.Buffer
	if err := configTemplate.Execute(&buf, pkgName); err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(configFilename, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("unable to write cfg file: " + err.Error())
	}

	return nil
}

func initSchema(schemaFilename string) error {
	_, err := os.Stat(schemaFilename)
	if !os.IsNotExist(err) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(schemaFilename), 0755); err != nil {
		return fmt.Errorf("unable to create schema dir: " + err.Error())
	}

	if err = ioutil.WriteFile(schemaFilename, []byte(strings.TrimSpace(schemaDefault)), 0644); err != nil {
		return fmt.Errorf("unable to write schema file: " + err.Error())
	}
	return nil
}
