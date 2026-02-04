# Instructions for agents

- Use `make help` to find available development targets
- Before running `.go` tests, run `make deps-backend` to install dependencies
- Before running `.ts` tests, run `make deps-frontend` to install dependencies
- Before committing `.go`, run `make fmt` to format, and run `make lint-go` to lint
- Before committing `.ts` changes, run `make lint-js` to lint
- Before committing `go.mod` changes, run `make tidy`
- Before committing new `.go` files, add the current year into the copyright header
- Before committing any files, remove all trailing whitespace from source code lines
