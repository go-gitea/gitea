# Instructions for agents

- Use `make help` to find available development targets
- Use the latest Go stable release when working on go code
- Use the latest Node.js LTS release when working on typescript code
- Before committing `.go` changes, run `make fmt` to format
- Before committing `.go` changes, run `make lint-go` to lint
- Before committing `.ts` changes, run `make lint-js` to lint
- Before committing `go.mod` changes, run `make tidy`
- Before committing new `.go` files, add the current year into the copyright header
- Before committing any files, removed all trailing whitespace from lines
