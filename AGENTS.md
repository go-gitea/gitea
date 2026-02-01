# Instructions for agents

- Use `make help` to find available development targets
- Use the latest go stable release when working on go code
- Use the latest Node.js LTS release when working on typescript code
- Before committing `.go` changes, run `make fmt` to format
- Before committing `.go` changes, run `make lint-go` to lint
- Before committing `.ts` changes, run `make lint-js` to lint
- Before committing `go.mod` changes, run `make tidy`
- Before committing new `.go` files, add the current year into the copyright header
- Before committing any files, removed any trailing whitespace
- Wait for up to 2 minutes for `go test <module>` commands to complete
- Wait for up to 5 minutes for `make lint-go` to complete
