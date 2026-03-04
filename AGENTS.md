# Instructions for agents

- Use `make help` to find available development targets
- Before committing `.go` changes, run `make fmt` to format, and run `make lint-go` to lint
- Before committing `.ts` changes, run `make lint-js` to lint
- Before committing `go.mod` changes, run `make tidy`
- Before committing new `.go` files, add the current year into the copyright header
- Before committing any files, remove all trailing whitespace from source code lines
- Never force-push to pull request branches
- Always start issue and pull request comments with an authorship attribution
