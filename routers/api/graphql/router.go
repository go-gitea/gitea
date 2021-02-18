package graphql

import (
	"net/http"

	"code.gitea.io/gitea/modules/graphql"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
)

// Handler Defining the Graphql handler
func Handler(w http.ResponseWriter, r *http.Request) {
	// NewExecutableSchema and Config are in the generated.go file
	// Resolver is in the resolver.go file
	h := handler.NewDefaultServer(graphql.NewExecutableSchema(
		graphql.Config{
			Resolvers: &graphql.Resolver{},
		}))

	h.ServeHTTP(w, r)
}

// PlaygroundHandler the Playground handler
func PlaygroundHandler(w http.ResponseWriter, r *http.Request) {
	h := playground.Handler("gitea graphql api", "/api/graphql")
	h.ServeHTTP(w, r)
}
