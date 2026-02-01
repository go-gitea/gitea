# Instructions for agents

- Use `make help` to find available development targets
- Before committing `.go` changes, run `make fmt`
- Before committing `.go` changes, run `make lint-go`
- Before committing `.ts` changes, run `make lint-js`
- Before committing `go.mod` changes, run `make tidy`
- Before committing new `.go` files, add the current year into the copyright header
- Before committing files, removed any trailing whitespace
- Wait for up to 5 minutes for `go test` and `lint-go` to complete
